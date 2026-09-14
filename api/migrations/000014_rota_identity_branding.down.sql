-- Organization branding is a retained capability. Refuse a downgrade that
-- would silently discard configured values or queued mail snapshots.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM auth_system_identity WHERE btrim(organization_name) <> '') THEN
        RAISE EXCEPTION 'cannot downgrade identity migration while organization branding is configured';
    END IF;
    IF EXISTS (SELECT 1 FROM auth_mail_outbox WHERE btrim(organization_name) <> '') THEN
        RAISE EXCEPTION 'cannot downgrade identity migration while mail snapshots contain organization branding';
    END IF;
END
$$;

ALTER TABLE auth_mail_outbox
    DROP CONSTRAINT IF EXISTS auth_mail_outbox_organization_name_check,
    DROP COLUMN IF EXISTS organization_name;
ALTER TABLE auth_system_identity
    DROP CONSTRAINT IF EXISTS auth_system_identity_organization_name_length,
    DROP COLUMN IF EXISTS organization_name;
