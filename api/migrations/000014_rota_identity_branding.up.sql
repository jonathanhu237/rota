-- Retain Rota's optional organization branding in Temvia's authoritative
-- system identity and durable mail-task snapshot. There is intentionally no
-- second branding table or service.
ALTER TABLE auth_system_identity
    ADD COLUMN organization_name text NOT NULL DEFAULT '',
    ADD CONSTRAINT auth_system_identity_organization_name_length
        CHECK (char_length(organization_name) <= 100);

ALTER TABLE auth_mail_outbox
    ADD COLUMN organization_name text NOT NULL DEFAULT '',
    ADD CONSTRAINT auth_mail_outbox_organization_name_check
        CHECK (char_length(organization_name) <= 100 AND organization_name !~ '[\r\n]');
