-- Only scripts/test-business-browser.sh loads this fixture into its newly
-- created disposable database. The administrator is initialized via real setup.
BEGIN;
INSERT INTO auth_users (id, name, email, email_canonical, password_hash, locale)
SELECT fixture.id::uuid, fixture.name, fixture.email, fixture.email, admin.password_hash, fixture.locale
FROM auth_users admin
CROSS JOIN (VALUES
 ('019535d9-3df7-79fb-b466-fa907fa17f91', 'Employee A', 'employee-a@example.com', 'en'),
 ('019535d9-3df7-79fb-b466-fa907fa17f92', 'Employee B', 'employee-b@example.com', 'zh-CN')
) AS fixture(id, name, email, locale)
WHERE admin.email = 'admin@example.com';
-- Temvia requires a nonempty role. The approved employee capability carries
-- no administrative read or management grants.
INSERT INTO auth_roles (id, name, name_canonical, description)
VALUES ('019535d9-3df7-79fb-b466-fa907fa17f93', 'Employee', 'employee', 'Self-service only');
INSERT INTO auth_role_permissions (role_id, permission_key)
VALUES ('019535d9-3df7-79fb-b466-fa907fa17f93', 'rota.self');
INSERT INTO auth_user_roles (user_id, role_id)
VALUES ('019535d9-3df7-79fb-b466-fa907fa17f91', '019535d9-3df7-79fb-b466-fa907fa17f93'),
       ('019535d9-3df7-79fb-b466-fa907fa17f92', '019535d9-3df7-79fb-b466-fa907fa17f93');
INSERT INTO positions (id, name) VALUES (9001, 'Browser position');
INSERT INTO templates (id, name) VALUES (9001, 'Browser template');
INSERT INTO template_slots (id, template_id, start_time, end_time)
VALUES (9001, 9001, '09:00', '10:00'), (9002, 9001, '11:00', '12:00');
INSERT INTO template_slot_weekdays (slot_id, weekday)
SELECT slot, weekday FROM unnest(ARRAY[9001,9002]) slot CROSS JOIN generate_series(1,7) weekday;
INSERT INTO template_slot_positions (slot_id, position_id, required_headcount, attendance_responsible)
VALUES (9001, 9001, 1, true), (9002, 9001, 1, true);
INSERT INTO user_positions (user_id, position_id)
VALUES ('019535d9-3df7-79fb-b466-fa907fa17f91', 9001), ('019535d9-3df7-79fb-b466-fa907fa17f92', 9001);
INSERT INTO publications (id, template_id, name, state, submission_start_at, submission_end_at, planned_active_from, planned_active_until)
VALUES (9001, 9001, 'Browser publication', 'ACTIVE', now()-interval '4 days', now()-interval '3 days', now()-interval '2 days', now()+interval '14 days');
INSERT INTO assignments (id, publication_id, user_id, slot_id, weekday, position_id)
VALUES (9001, 9001, '019535d9-3df7-79fb-b466-fa907fa17f91', 9001, extract(isodow from current_date+2), 9001),
       (9002, 9001, '019535d9-3df7-79fb-b466-fa907fa17f92', 9002, extract(isodow from current_date+2), 9001);
COMMIT;
