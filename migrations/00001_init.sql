
CREATE TABLE users (
    "id"                bigint          GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "name"              varchar(32)     NOT NULL UNIQUE CHECK ("name" <> ''),
    "email"             varchar(128)    NOT NULL UNIQUE CHECK ("email" <> ''),

    "country"           char(2)         NOT NULL,
    "language"          char(5)         NOT NULL,
    "timezone"          varchar(64)     NOT NULL CHECK ("timezone" <> ''),

    "created_at"        timestamptz     NOT NULL DEFAULT now(),
    "updated_at"        timestamptz     NOT NULL DEFAULT now()
);


CREATE TABLE addresses (
    "id"                bigint          GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "user"              bigint          NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    "line1"             text            NOT NULL CHECK ("line1" <> ''),
    "line2"             text            NOT NULL,
    "city"              text            NOT NULL CHECK ("city" <> ''),
    -- https://github.com/amckenna41/iso3166-2/tree/main/iso3166_2_resources
    "state"             char(16)        NOT NULL,
    "country"           char(2)         NOT NULL CHECK ("country" <> ''),
    "zip"               varchar(32)     NOT NULL,

    "created_at"        timestamptz     NOT NULL DEFAULT now(),
    "updated_at"        timestamptz     NOT NULL DEFAULT now()
);


CREATE TABLE releases (
    "id"                bigint          GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "product"           varchar(33)     NOT NULL CHECK ("product" <> ''),
    "version"           varchar(32)     NOT NULL CHECK ("version" <> ''),
    "retail_price"      integer         NOT NULL CHECK ("retail_price" >= 0),
    "notes"             text            NOT NULL,

    "created_at"        timestamptz     NOT NULL DEFAULT now(),
    "updated_at"        timestamptz     NOT NULL DEFAULT now()
);

CREATE TYPE order_status AS ENUM (
    -- The order is not paid yet. List of items can be adjusted.
    'draft',
    -- The customer is paying for the order or has paid for the order.
    -- We have'nt started the fulfillment yet.
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
    "address"           bigint          NULL REFERENCES addresses(id) ON DELETE SET NULL,
    "status"            order_status    NOT NULL DEFAULT 'draft',
    "paid"              boolean         NOT NULL DEFAULT false,

    "created_at"        timestamptz     NOT NULL DEFAULT now(),
    "updated_at"        timestamptz     NOT NULL DEFAULT now()
);


CREATE TABLE order_items (
    "id"                bigint          GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "order"             bigint          NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    "product"           varchar(33)     NOT NULL CHECK ("product" <> ''),
    "release"           bigint          NULL REFERENCES releases(id) ON DELETE SET NULL,
    "quantity"          integer         NOT NULL DEFAULT 1 CHECK ("quantity" >= 1),
    "retail_price"      integer         NOT NULL CHECK ("retail_price" >= 0),
    "fulfilled"         boolean         NOT NULL DEFAULT false,

    "created_at"        timestamptz     NOT NULL DEFAULT now(),
    "updated_at"        timestamptz     NOT NULL DEFAULT now()
);

CREATE TABLE owned_products (
    "id"                bigint          GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "user"              bigint          NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    "product"           varchar(33)     NOT NULL CHECK ("product" <> ''),
    "quantity"          integer         NOT NULL DEFAULT 1 CHECK ("quantity" >= 1),
    "release"           bigint          NULL REFERENCES releases(id) ON DELETE SET NULL,

    "created_at"        timestamptz     NOT NULL DEFAULT now(),
    "updated_at"        timestamptz     NOT NULL DEFAULT now()
)
