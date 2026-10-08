-- Records owner/admin rescue access to restricted business objects. This is
-- intentionally an audit log, not an authorization grant table: visibility is
-- derived from the current issue/project/squad and agent-access relationships.
CREATE TABLE object_visibility_audit (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    actor_user_id UUID NOT NULL,
    object_type TEXT NOT NULL CHECK (object_type IN ('issue', 'project', 'squad')),
    object_id UUID NOT NULL,
    action TEXT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
