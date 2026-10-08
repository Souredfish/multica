package handler

import (
	"context"
	"testing"
)

func TestRecordRescueObjectAudit(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	scope, ok := testHandler.objectVisibilityForMember(ctx, testWorkspaceID, testUserID)
	if !ok || !scope.rescue {
		t.Fatal("integration fixture must be a workspace owner")
	}
	objectID := scope.userID
	const action = "audit_insert_regression"
	if err := testHandler.recordRescueObject(ctx, scope, "issue", objectID, action); err != nil {
		t.Fatalf("insert visibility audit row: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testHandler.DB.Exec(ctx, `DELETE FROM object_visibility_audit WHERE workspace_id = $1 AND actor_user_id = $2 AND object_id = $3 AND action = $4`, scope.workspaceID, scope.userID, objectID, action)
	})
}
