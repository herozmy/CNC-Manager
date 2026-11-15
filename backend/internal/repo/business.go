package repo

import (
	"cnc-manager/internal/domain"
	"context"
)

func (r *Repo) ListDrawings(ctx context.Context, keyword string, page, size int) (domain.Page[domain.Drawing], error) {
	return businessTransaction(ctx, r, "read", "", 0, func(t *Repo) (domain.Page[domain.Drawing], error) { return t.rawListDrawings(ctx, keyword, page, size) })
}
func (r *Repo) GetDrawing(ctx context.Context, id int64) (*domain.Drawing, error) {
	return businessTransaction(ctx, r, "read", "drawing", id, func(t *Repo) (*domain.Drawing, error) { return t.rawGetDrawing(ctx, id) })
}
func (r *Repo) DrawingDetail(ctx context.Context, id int64) (*domain.DrawingDetail, error) {
	return businessTransaction(ctx, r, "read", "drawing", id, func(t *Repo) (*domain.DrawingDetail, error) { return t.rawDrawingDetail(ctx, id) })
}
func (r *Repo) CreateDrawing(ctx context.Context, in domain.DrawingInput) (*domain.Drawing, error) {
	return businessTransaction(ctx, r, "create", "", 0, func(t *Repo) (*domain.Drawing, error) { return t.rawCreateDrawing(ctx, in) })
}
func (r *Repo) UpdateDrawing(ctx context.Context, id int64, in domain.DrawingInput) (*domain.Drawing, error) {
	return businessTransaction(ctx, r, "edit", "drawing", id, func(t *Repo) (*domain.Drawing, error) { return t.rawUpdateDrawing(ctx, id, in) })
}
func (r *Repo) DeleteDrawing(ctx context.Context, id int64) error {
	_, err := businessTransaction(ctx, r, "delete", "drawing", id, func(t *Repo) (struct{}, error) { return struct{}{}, t.rawDeleteDrawing(ctx, id) })
	return err
}
func (r *Repo) ListOperations(ctx context.Context, drawingID int64) ([]domain.Operation, error) {
	return businessTransaction(ctx, r, "read", "drawing", drawingID, func(t *Repo) ([]domain.Operation, error) { return t.rawListOperations(ctx, drawingID) })
}
func (r *Repo) GetOperation(ctx context.Context, id int64) (*domain.Operation, error) {
	return businessTransaction(ctx, r, "read", "operation", id, func(t *Repo) (*domain.Operation, error) { return t.rawGetOperation(ctx, id) })
}
func (r *Repo) CreateOperation(ctx context.Context, drawingID int64, in domain.OperationInput) (*domain.Operation, error) {
	return businessTransaction(ctx, r, "create", "drawing", drawingID, func(t *Repo) (*domain.Operation, error) { return t.rawCreateOperation(ctx, drawingID, in) })
}
func (r *Repo) UpdateOperation(ctx context.Context, id int64, in domain.OperationInput) (*domain.Operation, error) {
	return businessTransaction(ctx, r, "edit", "operation", id, func(t *Repo) (*domain.Operation, error) { return t.rawUpdateOperation(ctx, id, in) })
}
func (r *Repo) DeleteOperation(ctx context.Context, id int64) error {
	_, err := businessTransaction(ctx, r, "delete", "operation", id, func(t *Repo) (struct{}, error) { return struct{}{}, t.rawDeleteOperation(ctx, id) })
	return err
}
func (r *Repo) ReorderOperations(ctx context.Context, drawingID int64, ids []int64) error {
	_, err := businessTransaction(ctx, r, "edit", "drawing", drawingID, func(t *Repo) (struct{}, error) { return struct{}{}, t.rawReorderOperations(ctx, drawingID, ids) })
	return err
}
func (r *Repo) ListPrograms(ctx context.Context, operationID int64) ([]domain.Program, error) {
	return businessTransaction(ctx, r, "read", "operation", operationID, func(t *Repo) ([]domain.Program, error) { return t.rawListPrograms(ctx, operationID) })
}
func (r *Repo) GetProgram(ctx context.Context, id int64) (*domain.Program, error) {
	return businessTransaction(ctx, r, "read", "program", id, func(t *Repo) (*domain.Program, error) { return t.rawGetProgram(ctx, id) })
}
func (r *Repo) CreateProgram(ctx context.Context, operationID int64, in domain.ProgramInput) (*domain.Program, error) {
	return businessTransaction(ctx, r, "create", "operation", operationID, func(t *Repo) (*domain.Program, error) { return t.rawCreateProgram(ctx, operationID, in) })
}
func (r *Repo) UpdateProgram(ctx context.Context, id int64, in domain.ProgramInput) (*domain.Program, error) {
	return businessTransaction(ctx, r, "edit", "program", id, func(t *Repo) (*domain.Program, error) { return t.rawUpdateProgram(ctx, id, in) })
}
func (r *Repo) DeleteProgram(ctx context.Context, id int64) error {
	_, err := businessTransaction(ctx, r, "delete", "program", id, func(t *Repo) (struct{}, error) { return struct{}{}, t.rawDeleteProgram(ctx, id) })
	return err
}
func (r *Repo) ReorderPrograms(ctx context.Context, operationID int64, ids []int64) error {
	_, err := businessTransaction(ctx, r, "edit", "operation", operationID, func(t *Repo) (struct{}, error) { return struct{}{}, t.rawReorderPrograms(ctx, operationID, ids) })
	return err
}
func (r *Repo) ListVersions(ctx context.Context, programID int64) ([]domain.Version, error) {
	return businessTransaction(ctx, r, "read", "program", programID, func(t *Repo) ([]domain.Version, error) { return t.rawListVersions(ctx, programID) })
}
func (r *Repo) GetVersion(ctx context.Context, id int64) (*domain.Version, error) {
	return businessTransaction(ctx, r, "read", "version", id, func(t *Repo) (*domain.Version, error) { return t.rawGetVersion(ctx, id) })
}
func (r *Repo) GetVersionFile(ctx context.Context, id int64) (*domain.VersionFile, error) {
	return businessTransaction(ctx, r, "read", "version", id, func(t *Repo) (*domain.VersionFile, error) { return t.rawGetVersionFile(ctx, id) })
}
func (r *Repo) AddVersion(ctx context.Context, programID int64, blob domain.BlobInput, fileName, changeNote string) (*domain.Version, error) {
	return businessTransaction(ctx, r, "edit", "program", programID, func(t *Repo) (*domain.Version, error) {
		return t.rawAddVersion(ctx, programID, blob, fileName, changeNote)
	})
}
func (r *Repo) OverwriteVersion(ctx context.Context, versionID int64, blob domain.BlobInput, expectedSHA256 *string) (*domain.Version, error) {
	return businessTransaction(ctx, r, "edit", "version", versionID, func(t *Repo) (*domain.Version, error) {
		return t.rawOverwriteVersion(ctx, versionID, blob, expectedSHA256)
	})
}
func (r *Repo) SetCurrentVersion(ctx context.Context, programID, versionID int64) (*domain.Program, error) {
	return businessTransaction(ctx, r, "edit", "program", programID, func(t *Repo) (*domain.Program, error) { return t.rawSetCurrentVersion(ctx, programID, versionID) })
}
func (r *Repo) GetVersionTools(ctx context.Context, versionID int64) (*domain.ToolTable, error) {
	return businessTransaction(ctx, r, "read", "version", versionID, func(t *Repo) (*domain.ToolTable, error) { return t.rawGetVersionTools(ctx, versionID) })
}
func (r *Repo) ReplaceVersionTools(ctx context.Context, versionID int64, in domain.ToolTableInput) (*domain.ToolTable, error) {
	return businessTransaction(ctx, r, "edit", "version", versionID, func(t *Repo) (*domain.ToolTable, error) { return t.rawReplaceVersionTools(ctx, versionID, in) })
}
