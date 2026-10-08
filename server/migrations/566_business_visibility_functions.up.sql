CREATE OR REPLACE FUNCTION can_member_read_issue(p_workspace_id UUID, p_issue_id UUID, p_user_id UUID)
RETURNS BOOLEAN
LANGUAGE SQL
STABLE
AS $$
SELECT EXISTS (
    SELECT 1
    FROM issue i
    JOIN member m ON m.workspace_id = i.workspace_id AND m.user_id = p_user_id
    WHERE i.workspace_id = p_workspace_id AND i.id = p_issue_id
      AND (
        m.role IN ('owner', 'admin')
        OR (i.creator_type = 'member' AND i.creator_id = p_user_id)
        OR EXISTS (
            SELECT 1 FROM issue_subscriber sub
            WHERE sub.issue_id = i.id AND sub.user_type = 'member' AND sub.user_id = p_user_id
        )
        OR (i.assignee_type IS DISTINCT FROM 'agent' AND i.assignee_type IS DISTINCT FROM 'squad')
        OR (i.assignee_type = 'agent' AND EXISTS (
            SELECT 1 FROM agent a
            WHERE a.id = i.assignee_id AND a.workspace_id = i.workspace_id
              AND (a.owner_id = p_user_id OR (a.permission_mode = 'public_to' AND EXISTS (
                  SELECT 1 FROM agent_invocation_target t
                  WHERE t.agent_id = a.id
                    AND (t.target_type = 'workspace' OR (t.target_type = 'member' AND t.target_id = p_user_id))
              )))
        ))
        OR (i.assignee_type = 'squad' AND EXISTS (
            SELECT 1 FROM squad s
            WHERE s.id = i.assignee_id AND s.workspace_id = i.workspace_id
              AND (
                EXISTS (SELECT 1 FROM squad_member sm WHERE sm.squad_id = s.id AND sm.member_type = 'member' AND sm.member_id = p_user_id)
                OR EXISTS (
                    SELECT 1 FROM agent a
                    WHERE a.id = s.leader_id AND a.workspace_id = s.workspace_id
                      AND (a.owner_id = p_user_id OR (a.permission_mode = 'public_to' AND EXISTS (
                          SELECT 1 FROM agent_invocation_target t
                          WHERE t.agent_id = a.id
                            AND (t.target_type = 'workspace' OR (t.target_type = 'member' AND t.target_id = p_user_id))
                      )))
                )
              )
        ))
      )
);
$$;

CREATE OR REPLACE FUNCTION can_member_read_project(p_workspace_id UUID, p_project_id UUID, p_user_id UUID)
RETURNS BOOLEAN
LANGUAGE SQL
STABLE
AS $$
SELECT EXISTS (
    SELECT 1
    FROM project p
    JOIN member m ON m.workspace_id = p.workspace_id AND m.user_id = p_user_id
    WHERE p.workspace_id = p_workspace_id AND p.id = p_project_id
      AND (
        m.role IN ('owner', 'admin')
        OR (p.lead_type = 'member' AND p.lead_id = p_user_id)
        OR (p.lead_type = 'agent' AND EXISTS (
            SELECT 1 FROM agent a
            WHERE a.id = p.lead_id AND a.workspace_id = p.workspace_id
              AND (a.owner_id = p_user_id OR (a.permission_mode = 'public_to' AND EXISTS (
                  SELECT 1 FROM agent_invocation_target t
                  WHERE t.agent_id = a.id
                    AND (t.target_type = 'workspace' OR (t.target_type = 'member' AND t.target_id = p_user_id))
              )))
        ))
        OR EXISTS (
            SELECT 1 FROM issue i
            WHERE i.workspace_id = p.workspace_id AND i.project_id = p.id
              AND can_member_read_issue(i.workspace_id, i.id, p_user_id)
        )
      )
);
$$;

CREATE OR REPLACE FUNCTION can_member_read_squad(p_workspace_id UUID, p_squad_id UUID, p_user_id UUID)
RETURNS BOOLEAN
LANGUAGE SQL
STABLE
AS $$
SELECT EXISTS (
    SELECT 1
    FROM squad s
    JOIN member m ON m.workspace_id = s.workspace_id AND m.user_id = p_user_id
    WHERE s.workspace_id = p_workspace_id AND s.id = p_squad_id
      AND (
        m.role IN ('owner', 'admin')
        OR EXISTS (SELECT 1 FROM squad_member sm WHERE sm.squad_id = s.id AND sm.member_type = 'member' AND sm.member_id = p_user_id)
        OR EXISTS (
            SELECT 1 FROM agent a
            WHERE a.id = s.leader_id AND a.workspace_id = s.workspace_id
              AND (a.owner_id = p_user_id OR (a.permission_mode = 'public_to' AND EXISTS (
                  SELECT 1 FROM agent_invocation_target t
                  WHERE t.agent_id = a.id
                    AND (t.target_type = 'workspace' OR (t.target_type = 'member' AND t.target_id = p_user_id))
              )))
        )
      )
);
$$;
