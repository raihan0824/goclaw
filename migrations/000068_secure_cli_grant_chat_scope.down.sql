DROP INDEX IF EXISTS idx_scag_chat;
DROP INDEX IF EXISTS idx_scag_unique_binary_agent_chat_tenant;

ALTER TABLE secure_cli_agent_grants
    ADD CONSTRAINT secure_cli_agent_grants_binary_id_agent_id_tenant_id_key
    UNIQUE (binary_id, agent_id, tenant_id);

ALTER TABLE secure_cli_agent_grants DROP COLUMN chat_id;
