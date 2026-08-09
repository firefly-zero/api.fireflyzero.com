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
    "stripe_id"         varchar(128)    NULL DEFAULT NULL UNIQUE CHECK ("stripe_id" <> ''),
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


-- Specific products added in an order.
CREATE TABLE order_items (
    "id"                bigint          GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "order"             bigint          NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    "product"           varchar(33)     NOT NULL CHECK ("product" <> ''),
    "quantity"          integer         NOT NULL DEFAULT 1 CHECK ("quantity" >= 1),
    "retail_price"      integer         NOT NULL CHECK ("retail_price" >= 0),
    "fulfilled"         boolean         NOT NULL DEFAULT false,

    "created_at"        timestamptz     NOT NULL DEFAULT now(),
    "updated_at"        timestamptz     NOT NULL DEFAULT now(),

    UNIQUE ("order", "product")
);

-- Paid items that the user bought or free items that the user downloaded.
CREATE TABLE owned_products (
    "id"                bigint          GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "user"              bigint          NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    "product"           varchar(33)     NOT NULL CHECK ("product" <> ''),
    "quantity"          integer         NOT NULL DEFAULT 1 CHECK ("quantity" >= 1),

    "created_at"        timestamptz     NOT NULL DEFAULT now(),
    "updated_at"        timestamptz     NOT NULL DEFAULT now(),

    UNIQUE ("user", "quantity")
);

CREATE TABLE groups (
    "slug"              varchar(33)     NOT NULL UNIQUE CHECK ("slug" <> ''),
    "name"              varchar(120)    NOT NULL CHECK ("name" <> ''),

    "created_at"        timestamptz     NOT NULL DEFAULT now(),
    "updated_at"        timestamptz     NOT NULL DEFAULT now()
);

CREATE TABLE products (
    "slug"              varchar(33)     NOT NULL UNIQUE CHECK ("slug" <> ''),
    "group"             varchar(120)    NOT NULL DEFAULT '',
    "name"              varchar(120)    NOT NULL CHECK ("name" <> ''),
    -- Common (for us) tax codes:
    --   * txcd_10201000: Video Games - downloaded - non subscription - with permanent rights
    --   * txcd_90000001: Cash Donation
    --   * txcd_34022001: Video Gaming Console - Portable
    --   * txcd_30011000: Clothing & Footwear
    -- More: https://docs.stripe.com/tax/tax-codes
    "tax_code"          varchar(120)    NOT NULL CHECK ("tax_code" <> ''),
    "retail_price"      integer         NOT NULL CHECK ("retail_price" >= 0),

    "created_at"        timestamptz     NOT NULL DEFAULT now(),
    "updated_at"        timestamptz     NOT NULL DEFAULT now()
);

INSERT INTO "groups" ("slug", "name") VALUES ('t-shirt', 'T-shirt');

INSERT INTO "products"
("slug",             "group",   "name",             "tax_code", "retail_price")
VALUES
('donation',         '',        '♥️ Donate',        'txcd_90000001',     0),
('device',           '',        '💡 Firefly Zero',  'txcd_34022001', 10000),
('t-shirt-xs-black', 't-shirt', '⚫ Black (XS)',    'txcd_30011000',  2500),
('t-shirt-s-black',  't-shirt', '⚫ Black (S)',     'txcd_30011000',  2500),
('t-shirt-m-black',  't-shirt', '⚫ Black (M)',     'txcd_30011000',  2500),
('t-shirt-l-black',  't-shirt', '⚫ Black (L)',     'txcd_30011000',  2500),
('t-shirt-xs-white', 't-shirt', '⚪️ White (XS)',    'txcd_30011000',  2500),
('t-shirt-s-white',  't-shirt', '⚪️ White (S)',     'txcd_30011000',  2500),
('t-shirt-m-white',  't-shirt', '⚪️ White (M)',     'txcd_30011000',  2500),
('t-shirt-l-white',  't-shirt', '⚪️ White (L)',     'txcd_30011000',  2500);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE products;
DROP TABLE groups;
DROP TABLE owned_products;
DROP TABLE order_items;
DROP TABLE orders;
DROP TABLE users;
DROP TYPE order_status;
-- +goose StatementEnd
