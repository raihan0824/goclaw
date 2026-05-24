-- Per-grant chat scoping for secure CLI grants.
-- Same agent, same binary, different env per inbound chat (e.g. WhatsApp group).
-- chat_id IS NULL = applies to every chat (backwards-compatible default).
-- chat_id = '...'  = applies only to that chat (more specific; wins over NULL at lookup).

ALTER TABLE secure_cli_agent_grants
    ADD COLUMN chat_id TEXT;

-- Replace the (binary_id, agent_id, tenant_id) uniqueness with one that includes chat_id.
-- COALESCE so NULL chat_id rows still enforce one-default-per-(binary,agent,tenant).
ALTER TABLE secure_cli_agent_grants
    DROP CONSTRAINT IF EXISTS secure_cli_agent_grants_binary_id_agent_id_tenant_id_key;

CREATE UNIQUE INDEX idx_scag_unique_binary_agent_chat_tenant
    ON secure_cli_agent_grants(binary_id, agent_id, COALESCE(chat_id, ''), tenant_id);

CREATE INDEX idx_scag_chat ON secure_cli_agent_grants(chat_id)
    WHERE chat_id IS NOT NULL;
