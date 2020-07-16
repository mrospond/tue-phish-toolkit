
-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
CREATE TABLE IF NOT EXISTS "fields" ("id" integer primary key autoincrement,"user_id" bigint,"name" varchar(50) NOT NULL UNIQUE,"modified_date" datetime );
CREATE TABLE IF NOT EXISTS "target_fields" ("id" integer primary key autoincrement,"field_id" integer NOT NULL,"target_id" integer NOT NULL,"value" varchar(255) NOT NULL );
CREATE TABLE IF NOT EXISTS "variables" ("id" integer primary key autoincrement,"field_id" integer NOT NULL DEFAULT 0,"user_id" bigint,"name" varchar(50) NOT NULL UNIQUE,"modified_date" datetime );
CREATE TABLE IF NOT EXISTS "conditions" ("id" integer primary key autoincrement,"variable_id" integer NOT NULL,"condition" text NOT NULL,"value" text NOT NULL );


-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP TABLE "fields";
DROP TABLE "targets_variables";
DROP TABLE "variables";
DROP TABLE "conditions";
