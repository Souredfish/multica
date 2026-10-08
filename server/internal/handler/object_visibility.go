package handler

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// objectVisibility is the requester's effective business-object scope. Agent
// invocation permission is an input to this scope, not the visibility model
// itself. Keeping the derived IDs together makes the same policy usable by
// single-object checks and SQL collection filters.
type objectVisibility struct {
	userID          pgtype.UUID
	workspaceID     pgtype.UUID
	rescue           bool
}

func (h *Handler) canReadIssue(ctx context.Context, s objectVisibility, issue db.Issue) bool {
	if s.rescue {
		return h.auditRescueObject(ctx, s, "issue", issue.ID, "read_or_write")
	}
	var allowed bool
	err := h.DB.QueryRow(ctx, "SELECT can_member_read_issue($1, $2, $3)", s.workspaceID, issue.ID, s.userID).Scan(&allowed)
	return err == nil && allowed
}

func (h *Handler) canReadProject(ctx context.Context, s objectVisibility, project db.Project) bool {
	if s.rescue {
		return h.auditRescueObject(ctx, s, "project", project.ID, "read_or_write")
	}
	var visible bool
	err := h.DB.QueryRow(ctx, "SELECT can_member_read_project($1, $2, $3)", s.workspaceID, project.ID, s.userID).Scan(&visible)
	return err == nil && visible
}

func (s objectVisibility) projectVisibilityPredicate(alias string, addArg func(any) string) string {
	if s.rescue {
		return "TRUE"
	}
	u := addArg(s.userID)
	return "can_member_read_project(" + alias + ".workspace_id, " + alias + ".id, " + u + "::uuid)"
}

func (h *Handler) canReadSquad(ctx context.Context, s objectVisibility, squad db.Squad) bool {
	if s.rescue {
		return h.auditRescueObject(ctx, s, "squad", squad.ID, "read_or_write")
	}
	var allowed bool
	err := h.DB.QueryRow(ctx, "SELECT can_member_read_squad($1, $2, $3)", s.workspaceID, squad.ID, s.userID).Scan(&allowed)
	return err == nil && allowed
}

// MemberCanReadBusinessObject exposes the unified visibility rule to event
// fanout, which runs outside the HTTP handler package boundary.
func (h *Handler) MemberCanReadBusinessObject(ctx context.Context, workspaceID, userID, objectType, objectID string) bool {
	scope, ok := h.objectVisibilityForMember(ctx, workspaceID, userID)
	if !ok {
		return false
	}
	id, err := util.ParseUUID(objectID)
	if err != nil {
		return false
	}
	ws, err := util.ParseUUID(workspaceID)
	if err != nil {
		return false
	}
	switch objectType {
	case "issue":
		issue, err := h.Queries.GetIssueInWorkspace(ctx, db.GetIssueInWorkspaceParams{ID: id, WorkspaceID: ws})
		return err == nil && h.canReadIssue(ctx, scope, issue)
	case "project":
		project, err := h.Queries.GetProjectInWorkspace(ctx, db.GetProjectInWorkspaceParams{ID: id, WorkspaceID: ws})
		return err == nil && h.canReadProject(ctx, scope, project)
	case "squad":
		squad, err := h.Queries.GetSquadInWorkspace(ctx, db.GetSquadInWorkspaceParams{ID: id, WorkspaceID: ws})
		return err == nil && h.canReadSquad(ctx, scope, squad)
	default:
		return false
	}
}

// BusinessObjectRecipients returns the current member audience for one
// business object. SQL and HTTP checks share the same database predicates.
func (h *Handler) BusinessObjectRecipients(ctx context.Context, workspaceID, objectType, objectID string) ([]string, error) {
	ws, err := util.ParseUUID(workspaceID)
	if err != nil {
		return nil, err
	}
	id, err := util.ParseUUID(objectID)
	if err != nil {
		return nil, err
	}
	function := ""
	switch objectType {
	case "issue":
		function = "can_member_read_issue"
	case "project":
		function = "can_member_read_project"
	case "squad":
		function = "can_member_read_squad"
	default:
		return nil, fmt.Errorf("unsupported business object type")
	}
	rows, err := h.DB.Query(ctx, fmt.Sprintf(`SELECT m.user_id, m.role
FROM member m
WHERE m.workspace_id = $1 AND %s($1, $2, m.user_id)`, function), ws, id)
	if err != nil {
		return nil, err
	}
	type audienceMember struct {
		userID pgtype.UUID
		role   string
	}
	members := make([]audienceMember, 0)
	for rows.Next() {
		var member audienceMember
		if err := rows.Scan(&member.userID, &member.role); err != nil {
			rows.Close()
			return nil, err
		}
		members = append(members, member)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	recipients := make([]string, 0, len(members))
	for _, member := range members {
		recipients = append(recipients, uuidToString(member.userID))
		if roleAllowed(member.role, "owner", "admin") {
			scope := objectVisibility{workspaceID: ws, userID: member.userID, rescue: true}
			if !h.auditRescueObject(ctx, scope, objectType, id, "realtime_delivery") {
				return nil, fmt.Errorf("record realtime rescue access")
			}
		}
	}
	return recipients, nil
}

func (h *Handler) issueEventRecipients(ctx context.Context, issue db.Issue) []string {
	members, err := h.Queries.ListMembers(ctx, issue.WorkspaceID)
	if err != nil {
		return nil
	}
	recipients := make([]string, 0, len(members))
	for _, member := range members {
		var allowed bool
		if err := h.DB.QueryRow(ctx, "SELECT can_member_read_issue($1, $2, $3)", issue.WorkspaceID, issue.ID, member.UserID).Scan(&allowed); err == nil && allowed {
			recipients = append(recipients, uuidToString(member.UserID))
		}
	}
	return recipients
}

func (h *Handler) auditRescueObject(ctx context.Context, scope objectVisibility, objectType string, objectID pgtype.UUID, action string) bool {
	err := h.recordRescueObject(ctx, scope, objectType, objectID, action)
	if err != nil {
		slog.Error("record object visibility rescue access failed", "workspace_id", uuidToString(scope.workspaceID), "actor_user_id", uuidToString(scope.userID), "object_type", objectType, "object_id", uuidToString(objectID), "action", action, "error", err)
	}
	return err == nil
}

func (h *Handler) recordRescueObject(ctx context.Context, scope objectVisibility, objectType string, objectID pgtype.UUID, action string) error {
	if !scope.rescue {
		return nil
	}
	_, err := h.DB.Exec(ctx, `INSERT INTO object_visibility_audit
    (workspace_id, actor_user_id, object_type, object_id, action)
VALUES ($1, $2, $3, $4, $5)`, scope.workspaceID, scope.userID, objectType, objectID, action)
	return err
}

func (h *Handler) visibleProjectIssueStats(ctx context.Context, workspaceID pgtype.UUID, projectIDs []pgtype.UUID, scope objectVisibility) (map[string]db.GetProjectIssueStatsRow, error) {
	result := make(map[string]db.GetProjectIssueStatsRow, len(projectIDs))
	if len(projectIDs) == 0 {
		return result, nil
	}
	terminalKeys := h.projectTerminalIssueStatusKeys(ctx, workspaceID)
	args := []any{workspaceID, projectIDs, terminalKeys}
	addArg := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}
	predicate := scope.issueVisibilityPredicate("i", addArg)
	rows, err := h.DB.Query(ctx, `SELECT i.project_id,
       COUNT(*)::bigint AS total_count,
       COUNT(*) FILTER (WHERE i.status = ANY($3::text[]))::bigint AS done_count
FROM issue i
WHERE i.workspace_id = $1 AND i.project_id = ANY($2::uuid[])
  AND `+predicate+`
GROUP BY i.project_id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var stats db.GetProjectIssueStatsRow
		if err := rows.Scan(&stats.ProjectID, &stats.TotalCount, &stats.DoneCount); err != nil {
			return nil, err
		}
		result[uuidToString(stats.ProjectID)] = stats
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (h *Handler) objectVisibilityForMember(ctx context.Context, workspaceID, userID string) (objectVisibility, bool) {
	ws, err := util.ParseUUID(workspaceID)
	if err != nil {
		return objectVisibility{}, false
	}
	user, err := util.ParseUUID(userID)
	if err != nil {
		return objectVisibility{}, false
	}
	member, err := h.Queries.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{UserID: user, WorkspaceID: ws})
	if err != nil {
		return objectVisibility{}, false
	}
	scope := objectVisibility{userID: user, workspaceID: ws, rescue: roleAllowed(member.Role, "owner", "admin")}
	if scope.rescue {
		return scope, true
	}

	return scope, true
}

// issueVisibilityPredicate returns a SQL predicate with arguments allocated by
// addArg. It deliberately keeps unrestricted workspace tasks visible, while
// restricting agent/squad assignments to the accepted subject mapping. The
// creator, explicit subscriber, and rescue rules are additive.
func (s objectVisibility) issueVisibilityPredicate(alias string, addArg func(any) string) string {
	if s.rescue {
		return "TRUE"
	}
	u := addArg(s.userID)
	return "can_member_read_issue(" + alias + ".workspace_id, " + alias + ".id, " + u + "::uuid)"
}
