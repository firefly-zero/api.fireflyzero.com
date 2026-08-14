-- +goose Up
-- +goose StatementBegin

CREATE TABLE users (
    "id"                bigint          GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "email"             varchar(128)    NOT NULL UNIQUE CHECK ("email" <> ''),
    -- ISO 3166-1 alpha-2 2-letter country code.
    -- https://en.wikipedia.org/wiki/ISO_639-1
    "country"           char(2)         NOT NULL,
    -- ISO 639-1 2-letter language code.
    -- https://en.wikipedia.org/wiki/ISO_3166-1_alpha-2
    "language"          char(2)         NOT NULL,
    -- Timezone identifier.
    -- https://en.wikipedia.org/wiki/List_of_tz_database_time_zones
    "timezone"          varchar(64)     NOT NULL CHECK ("timezone" <> ''),
    -- ID of the customer in Stripe.
    "stripe_id"         varchar(128)    NOT NULL UNIQUE CHECK ("stripe_id" <> ''),
    -- When the user was soft-deleted, if ever.
    "deleted_at"        timestamptz     NULL DEFAULT NULL,
    "created_at"        timestamptz     NOT NULL DEFAULT now(),
    "updated_at"        timestamptz     NOT NULL DEFAULT now()
);

CREATE TYPE order_status AS ENUM (
    -- The order is not paid yet. List of items can be adjusted.
    'draft',
    -- The customer is paying for the order or has paid for the order.
    -- We haven't started the fulfillment yet.
    'pending',
    -- We started to fulfill the order.
    -- The order cannot be canceled anymore.
    'in_process',
    -- All items in the order are fulfilled.
    'fulfilled',
    -- We failed to fulfill the order and need to resolve it with the customer.
    -- For example, if the shipping address is invalid.
    'on_hold',
    -- The order has been canceled.
    'canceled'
);


CREATE TABLE orders (
    "id"                bigint          GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "user"              bigint          NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    "status"            order_status    NOT NULL DEFAULT 'draft',
    "paid"              boolean         NOT NULL DEFAULT false,
    -- ID of the checkout session in Stripe.
    "stripe_id"         varchar(128)    NULL DEFAULT NULL UNIQUE CHECK ("stripe_id" <> ''),

    "created_at"        timestamptz     NOT NULL DEFAULT now(),
    "updated_at"        timestamptz     NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX orders_unique_draft ON "orders" ("user") WHERE ("status" = 'draft');

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE orders;
DROP TABLE users;
DROP TYPE order_status;
-- +goose StatementEnd
