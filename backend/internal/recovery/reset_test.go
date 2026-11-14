package recovery

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"cnc-manager/internal/auth"
	"cnc-manager/internal/datalock"
	"cnc-manager/internal/store"
)

type fixture struct {
	data, oldHash string
	account       Account
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	data := filepath.Join(t.TempDir(), "data")
	db, err := store.Open(filepath.Join(data, "cnc.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := store.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	hash, err := auth.HashPassword("Original2026!")
	if err != nil {
		t.Fatal(err)
	}
	for i, name := range []string{"admin", "other"} {
		if _, err := db.Exec("INSERT INTO app_user (id, username, display_name, password_hash, role, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, 'admin', 1, '2026-01-01', '2026-01-01')", i+1, name, "Name "+name, hash); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec("INSERT INTO user_session (id, token_hash, user_id, expires_at, created_at) VALUES (?, ?, ?, '2099-01-01', '2026-01-01')", i+1, "token-"+name, i+1); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec("INSERT INTO drawing (id, drawing_no, created_at, updated_at) VALUES (1, 'UNCHANGED', '2026-01-01', '2026-01-01')"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(data, "nc"), 0700); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{"nc.key": strings.Repeat("k", 32), "nc/fixture.nc": "G01 X1\r\nM30\r\n"} {
		if err := os.WriteFile(filepath.Join(data, name), []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return fixture{data, hash, Account{1, "admin", "Name admin", true}}
}

func execSQL(t *testing.T, data, query string) {
	t.Helper()
	db, err := openDB(data, "rw")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(query); err != nil {
		t.Fatal(err)
	}
}

func rows(t *testing.T, data, table string) [][]any {
	t.Helper()
	db, err := openDB(data, "ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	r, err := db.Query("SELECT * FROM " + table + " ORDER BY rowid")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	columns, err := r.Columns()
	if err != nil {
		t.Fatal(err)
	}
	var result [][]any
	for r.Next() {
		values, ptrs := make([]any, len(columns)), make([]any, len(columns))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := r.Scan(ptrs...); err != nil {
			t.Fatal(err)
		}
		result = append(result, values)
	}
	if err := r.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}

type fakeService struct {
	running, failStop, failStart bool
	data                         string
	calls                        []string
	onStop                       func()
}

func (s *fakeService) Running(context.Context) (bool, error) { return s.running, nil }
func (s *fakeService) Stop(context.Context) error {
	s.calls = append(s.calls, "stop")
	if s.onStop != nil {
		s.onStop()
	}
	if s.failStop {
		return errors.New("stop failed")
	}
	return nil
}
func (s *fakeService) Start(ctx context.Context) error {
	s.calls = append(s.calls, "start")
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.failStart {
		return errors.New("start failed")
	}
	lock, err := datalock.Acquire(filepath.Join(s.data, ".cnc-manager.lock"))
	if err != nil {
		return err
	}
	return lock.Close()
}

func TestResetBackupAndAccountIsolation(t *testing.T) {
	f := newFixture(t)
	before := map[string][][]any{}
	for _, table := range []string{"app_user", "user_session", "drawing", "audit_log"} {
		before[table] = rows(t, f.data, table)
	}
	service := &fakeService{running: true, data: f.data}
	var output bytes.Buffer
	r, err := Reset(context.Background(), f.data, f.account, "NewPassword2026!", service, &output)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Changed || !r.ServiceRestored || r.Revoked != 1 {
		t.Fatalf("result: %+v", r)
	}
	if !reflect.DeepEqual(service.calls, []string{"stop", "start"}) {
		t.Fatal(service.calls)
	}
	backup := filepath.Join(r.Backup, "data")
	for table, original := range before {
		if !reflect.DeepEqual(rows(t, backup, table), original) {
			t.Fatalf("backup changed %s", table)
		}
	}
	users := rows(t, f.data, "app_user")
	if !reflect.DeepEqual(users[1], before["app_user"][1]) {
		t.Fatal("other account changed")
	}
	if !reflect.DeepEqual(users[0][:3], before["app_user"][0][:3]) || !reflect.DeepEqual(users[0][4:7], before["app_user"][0][4:7]) {
		t.Fatal("account metadata changed")
	}
	if !auth.CheckPassword(users[0][3].(string), "NewPassword2026!") || auth.CheckPassword(users[0][3].(string), "Original2026!") {
		t.Fatal("incorrect password")
	}
	if !reflect.DeepEqual(rows(t, f.data, "user_session"), before["user_session"][1:]) {
		t.Fatal("wrong sessions revoked")
	}
	if !reflect.DeepEqual(rows(t, f.data, "drawing"), before["drawing"]) {
		t.Fatal("business data changed")
	}
	for _, name := range []string{"nc.key", "nc/fixture.nc"} {
		a, err := os.ReadFile(filepath.Join(f.data, name))
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(filepath.Join(backup, name))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(a, b) {
			t.Fatal("file changed", name)
		}
	}
	info, err := os.Stat(r.Backup)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0700 {
		t.Fatal("backup permissions", info.Mode())
	}
	audit := fmt.Sprint(rows(t, f.data, "audit_log"))
	if !strings.Contains(audit, "reset_password") || strings.Contains(audit+output.String(), "NewPassword2026!") {
		t.Fatal("audit missing or secret leaked")
	}
}

func TestResetFailuresPreservePasswordAndRecoverService(t *testing.T) {
	for _, scenario := range []string{"transaction", "audit", "backup", "stop", "cancel", "disabled"} {
		t.Run(scenario, func(t *testing.T) {
			f := newFixture(t)
			service := &fakeService{running: true, data: f.data}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch scenario {
			case "transaction":
				execSQL(t, f.data, "CREATE TRIGGER deny_delete BEFORE DELETE ON user_session BEGIN SELECT RAISE(ABORT,'fixture failure'); END")
			case "audit":
				execSQL(t, f.data, "CREATE TRIGGER deny_audit BEFORE INSERT ON audit_log BEGIN SELECT RAISE(ABORT,'fixture failure'); END")
			case "backup":
				if err := os.Symlink("nc.key", filepath.Join(f.data, "bad-link")); err != nil {
					t.Fatal(err)
				}
			case "stop":
				service.failStop = true
			case "cancel":
				service.onStop = cancel
			case "disabled":
				execSQL(t, f.data, "UPDATE app_user SET enabled=0 WHERE id=1")
			}
			r, err := Reset(ctx, f.data, f.account, "NewPassword2026!", service, io.Discard)
			if err == nil || r.Changed {
				t.Fatalf("expected failure: %+v %v", r, err)
			}
			if !r.ServiceRestored || !reflect.DeepEqual(service.calls, []string{"stop", "start"}) {
				t.Fatalf("service not recovered: %+v %v", r, service.calls)
			}
			if rows(t, f.data, "app_user")[0][3] != f.oldHash || len(rows(t, f.data, "user_session")) != 2 || len(rows(t, f.data, "audit_log")) != 0 {
				t.Fatal("partial transaction committed")
			}
		})
	}
}

func TestResetStartFailureReportsCommittedChange(t *testing.T) {
	f := newFixture(t)
	r, err := Reset(context.Background(), f.data, f.account, "NewPassword2026!", &fakeService{running: true, failStart: true, data: f.data}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "密码已重置，但服务恢复失败") || !r.Changed || r.ServiceRestored {
		t.Fatalf("result %+v %v", r, err)
	}
	if !auth.CheckPassword(rows(t, f.data, "app_user")[0][3].(string), "NewPassword2026!") {
		t.Fatal("password not changed")
	}
}

func TestResetOfflineNeverStartsService(t *testing.T) {
	f := newFixture(t)
	s := &fakeService{data: f.data}
	r, err := Reset(context.Background(), f.data, f.account, strings.Repeat("密", 128), s, io.Discard)
	if err != nil || !r.Changed || r.ServiceRestored || len(s.calls) != 0 {
		t.Fatalf("%+v %v %v", r, err, s.calls)
	}
}

func TestConcurrentResetAndRunningServerAreRejected(t *testing.T) {
	for _, kind := range []string{"maintenance", "server"} {
		t.Run(kind, func(t *testing.T) {
			f := newFixture(t)
			path := filepath.Join(f.data, ".cnc-manager.lock")
			if kind == "maintenance" {
				root := filepath.Join(filepath.Dir(f.data), "backups")
				if err := os.Mkdir(root, 0700); err != nil {
					t.Fatal(err)
				}
				path = filepath.Join(root, ".password-reset.lock")
			}
			lock, err := datalock.Acquire(path)
			if err != nil {
				t.Fatal(err)
			}
			defer lock.Close()
			r, err := Reset(context.Background(), f.data, f.account, "NewPassword2026!", nil, io.Discard)
			if err == nil || r.Changed || r.Backup != "" {
				t.Fatalf("%+v %v", r, err)
			}
			if rows(t, f.data, "app_user")[0][3] != f.oldHash {
				t.Fatal("password changed")
			}
		})
	}
}

func TestMissingDatabaseAndNonTerminalDoNotReset(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	if _, err := Accounts(context.Background(), missing); err == nil {
		t.Fatal("missing accepted")
	}
	if _, err := os.Stat(missing); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("directory created")
	}
	f := newFixture(t)
	in, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	args := []string{"--data-dir", f.data, "--no-service"}
	if err := Run(context.Background(), args, in, io.Discard); err == nil {
		t.Fatal("non-terminal accepted")
	}
	var out bytes.Buffer
	if err := Run(context.Background(), append(args, "--list"), in, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "admin") || !strings.Contains(out.String(), "other") {
		t.Fatal(out.String())
	}
	if err := Run(context.Background(), append(args, "secret-password"), in, io.Discard); err == nil {
		t.Fatal("positional password accepted")
	}
	if rows(t, f.data, "app_user")[0][3] != f.oldHash {
		t.Fatal("password changed")
	}
}

func TestSelectionAndPasswordInput(t *testing.T) {
	users := []Account{{1, "Admin", "A", true}, {2, "other", "B", true}, {3, "disabled", "C", false}, {4, "K", "D", true}}
	answers := func(values ...string) func(string) (string, error) {
		return func(string) (string, error) {
			if len(values) == 0 {
				return "", io.EOF
			}
			v := values[0]
			values = values[1:]
			return v, nil
		}
	}
	for _, tc := range []struct {
		name  string
		users []Account
		user  string
		input []string
		id    int64
		fail  bool
	}{
		{"single", users[:1], "", nil, 1, false}, {"case", users, "ADMIN", nil, 1, false},
		{"multiple", users, "", []string{"x", "9", "2"}, 2, false}, {"cancel", users, "", []string{"0"}, 0, false},
		{"disabled", users, "disabled", nil, 0, true}, {"unicode_not_folded", users, "k", nil, 0, true}, {"empty", nil, "", nil, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, err := selectAccount(tc.users, tc.user, answers(tc.input...), io.Discard)
			if (err != nil) != tc.fail {
				t.Fatal(err)
			}
			if tc.fail {
				return
			}
			if tc.id == 0 {
				if a != nil {
					t.Fatal(a)
				}
			} else if a == nil || a.ID != tc.id {
				t.Fatal(a)
			}
		})
	}
	p, err := readPassword(answers("short", "NewPassword2026!", "different", strings.Repeat("密", 128), strings.Repeat("密", 128)), io.Discard)
	if err != nil || p != strings.Repeat("密", 128) {
		t.Fatal("input validation", err)
	}
	f := newFixture(t)
	for _, p := range []string{strings.Repeat("密", 7), strings.Repeat("密", 129)} {
		if _, err := Reset(context.Background(), f.data, f.account, p, nil, io.Discard); err == nil {
			t.Fatal("length accepted")
		}
	}
}

func TestSystemdDirectoryValidation(t *testing.T) {
	data := filepath.Join(t.TempDir(), "data with space")
	if err := os.Mkdir(data, 0700); err != nil {
		t.Fatal(err)
	}
	for _, env := range []string{fmt.Sprintf("OTHER=value %q", "CNC_DATA_DIR="+data), fmt.Sprintf("%q OTHER=value", "CNC_DATA_DIR="+data)} {
		if err := matchDataDir(env, data); err != nil {
			t.Fatal(err)
		}
	}
	for _, env := range []string{"", "CNC_DATA_DIR=relative", "CNC_DATA_DIR=/different", `"CNC_DATA_DIR=unterminated`} {
		if err := matchDataDir(env, data); err == nil {
			t.Fatal("unsafe environment accepted", env)
		}
	}
}
