package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const defaultRetention = 7

var beijing = time.FixedZone("UTC+8", 8*60*60)

type scheduleSettings struct {
	Retention int          `json:"retention"`
	Frequency string       `json:"frequency"`
	Time      string       `json:"time"`
	Weekday   time.Weekday `json:"weekday"`
}

type backupSchedule struct {
	scheduleSettings
	Next       *time.Time `json:"next"`
	LastResult string     `json:"lastResult"`
}

func (p scheduleSettings) nextAfter(now time.Time) (*time.Time, error) {
	if p.Retention < 1 || p.Retention > 365 {
		return nil, errors.New("计划备份保留份数须为 1 至 365 的整数")
	}
	clock, err := time.Parse("15:04", p.Time)
	if err != nil || clock.Format("15:04") != p.Time || p.Weekday < 0 || p.Weekday > 6 {
		return nil, errors.New("请输入有效的备份时间和星期")
	}
	if p.Frequency == "off" {
		return nil, nil
	}
	if p.Frequency != "daily" && p.Frequency != "weekly" {
		return nil, errors.New("请选择每日、每周或关闭")
	}
	day := now.In(beijing)
	next := time.Date(day.Year(), day.Month(), day.Day(), clock.Hour(), clock.Minute(), 0, 0, beijing)
	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}
	if p.Frequency == "weekly" {
		next = next.AddDate(0, 0, (int(p.Weekday-next.Weekday())+7)%7)
	}
	return &next, nil
}

func (s *Server) readSchedule() (backupSchedule, error) {
	p := backupSchedule{scheduleSettings: scheduleSettings{Frequency: "off", Time: "02:00", Weekday: time.Monday, Retention: defaultRetention}}
	data, err := os.ReadFile(filepath.Join(s.backup.Layout.Root, "backup-schedule.json"))
	if os.IsNotExist(err) {
		return p, nil
	}
	if err == nil {
		err = json.Unmarshal(data, &p)
	}
	return p, err
}

func (s *Server) saveSchedule(p backupSchedule) error {
	root := s.backup.Layout.Root
	f, err := os.CreateTemp(root, ".backup-schedule-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	err = json.NewEncoder(f).Encode(p)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), filepath.Join(root, "backup-schedule.json"))
}

func (s *Server) handleGetSchedule(w http.ResponseWriter, _ *http.Request) {
	p, err := s.readSchedule()
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, 200, p)
}

func (s *Server) handleSetSchedule(w http.ResponseWriter, r *http.Request) {
	p, err := s.readSchedule()
	if err != nil {
		s.fail(w, err)
		return
	}
	// Older clients omit retention when editing the schedule.
	settings := scheduleSettings{Retention: p.Retention}
	if !decodeJSON(w, r, &settings) {
		return
	}
	next, err := settings.nextAfter(time.Now())
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	p.scheduleSettings = settings
	p.Next = next
	if err = s.saveSchedule(p); err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, 200, p)
}

func (s *Server) RunBackupSchedule(ctx context.Context) {
	timer := time.NewTicker(15 * time.Second)
	defer timer.Stop()
	for ctx.Err() == nil {
		if err := s.runDueBackup(ctx, time.Now()); err != nil {
			s.log.Error("计划备份失败", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
	}
}

func (s *Server) runDueBackup(ctx context.Context, now time.Time) error {
	// Read the atomically replaced file before queuing an exclusive maintenance lock.
	p, err := s.readSchedule()
	if err != nil || p.Next == nil || p.Next.After(now) {
		return err
	}
	s.maintenance.Lock()
	defer s.maintenance.Unlock()
	p, err = s.readSchedule()
	if err != nil || p.Next == nil || p.Next.After(now) {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	p.Next, err = p.nextAfter(now)
	if err != nil {
		return err
	}
	stamp := now.In(beijing).Format("2006-01-02 15:04")
	p.LastResult = stamp + " 已启动，结果待确认"
	if err = s.saveSchedule(p); err != nil {
		return err
	}
	path, runErr := s.backup.Retain(ctx, "scheduled")
	p.LastResult = stamp + " 备份成功"
	if runErr != nil {
		p.LastResult = stamp + " 备份失败，请查看服务日志"
	} else if runErr = s.backup.PruneScheduled(filepath.Base(path), p.Retention); runErr != nil {
		p.LastResult = stamp + " 备份成功，旧计划备份清理失败，请查看服务日志"
	}
	if err = s.saveSchedule(p); err != nil {
		return fmt.Errorf("保存执行结果: %w", errors.Join(runErr, err))
	}
	return runErr
}
