-- +goose Up
-- +goose StatementBegin

CREATE TABLE users (
    "id"                bigint          GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "email"             varchar(128)    NOT NULL UNIQUE CHECK ("email" <> ''),
    -- List of author IDs in the catalog that belong to the user.
    -- Multiple users may share the same name. A name from the catalog
    -- might not belong to any user in the shop.
    "author_ids"        varchar(16)[]   NOT NULL DEFAULT '{}',
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
    -- When was the last time that we transfered money to the author.
    "transfered_at"     timestamptz     NULL DEFAULT NULL,

    -- When the user was soft-deleted, if ever.
    "deleted_at"        timestamptz     NULL DEFAULT NULL,
    "created_at"        timestamptz     NOT NULL DEFAULT now(),
    "updated_at"        timestamptz     NOT NULL DEFAULT now()
);


CREATE TABLE addresses (
    "id"                bigint          GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "user"              bigint          NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    "line1"             varchar(256)    NOT NULL CHECK ("line1" <> ''),
    "line2"             varchar(256)    NOT NULL,
    "city"              varchar(256)    NOT NULL CHECK ("city" <> ''),
    -- https://github.com/amckenna41/iso3166-2/tree/main/iso3166_2_resources
    "state"             varchar(16)     NULL,
    "country"           char(2)         NOT NULL CHECK ("country" <> ''),
    "zip"               varchar(32)     NOT NULL,

    "created_at"        timestamptz     NOT NULL DEFAULT now(),
    "updated_at"        timestamptz     NOT NULL DEFAULT now()
);

-- Releases for apps and games.
CREATE TABLE releases (
    "id"                bigint          GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    -- The full ID of the app (author_id.app_id).
    "product"           varchar(33)     NOT NULL CHECK ("product" <> ''),
    -- Version number of the release (typically, SemVer).
    "version"           varchar(32)     NOT NULL CHECK ("version" <> ''),
    -- How much a customer would pay to get the product.
    -- The price of the latest release defines the price of the product.
    "retail_price"      integer         NOT NULL CHECK ("retail_price" >= 0),
    -- Release notes (Markdown). Can be empty.
    "notes"             text            NOT NULL,

    "created_at"        timestamptz     NOT NULL DEFAULT now(),
    "updated_at"        timestamptz     NOT NULL DEFAULT now(),

    UNIQUE ("product", "version")
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
    "address"           bigint          NULL DEFAULT NULL REFERENCES addresses(id) ON DELETE SET NULL,
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
    "release"           bigint          NULL REFERENCES releases(id) ON DELETE SET NULL,
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
    -- The latest downloaded release.
    -- We use it to highlight items with a newer release available.
    "release"           bigint          NULL REFERENCES releases(id) ON DELETE SET NULL,

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

CREATE TYPE transaction_kind AS ENUM (
    -- A customer paid for a product.
    'payment',
    -- We returned money to a customer.
    'reimbursement',
    'earning',
    'withdrawal',
);

CREATE TABLE transactions (
    "id"                bigint          GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "created_at"        timestamptz     NOT NULL DEFAULT now()
);

CREATE TABLE products (
    "slug"              varchar(33)     NOT NULL UNIQUE CHECK ("slug" <> ''),
    "group"             varchar(120)    NOT NULL DEFAULT '',
    "name"              varchar(120)    NOT NULL CHECK ("name" <> ''),
    "retail_price"      integer         NOT NULL CHECK ("retail_price" >= 0),

    "created_at"        timestamptz     NOT NULL DEFAULT now(),
    "updated_at"        timestamptz     NOT NULL DEFAULT now()
);

INSERT INTO "groups" ("slug", "name") VALUES ("t-shirt", "T-shirt");

INSERT INTO "products"
("slug", "group", "name", "retail_price")
VALUES
('donation',         '',        '♥️ Donate', 0),
('device',           '',        '💡 Firefly Zero', 10000),
('t-shirt-xs-black', 't-shirt', '⚫ Black (XS)', 2500),
('t-shirt-s-black',  't-shirt', '⚫ Black (S)',  2500),
('t-shirt-m-black',  't-shirt', '⚫ Black (M)',  2500),
('t-shirt-l-black',  't-shirt', '⚫ Black (L)',  2500),
('t-shirt-xs-white', 't-shirt', '⚪️ White (XS)', 2500),
('t-shirt-s-white',  't-shirt', '⚪️ White (S)',  2500),
('t-shirt-m-white',  't-shirt', '⚪️ White (M)',  2500),
('t-shirt-l-white',  't-shirt', '⚪️ White (L)',  2500);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE products;
DROP TABLE groups;
DROP TABLE owned_products;
DROP TABLE order_items;
DROP TABLE orders;
DROP TABLE releases;
DROP TABLE addresses;
DROP TABLE users;
DROP TYPE order_status;
-- +goose StatementEnd
