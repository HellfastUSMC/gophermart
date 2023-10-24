-- +goose Up
CREATE TABLE IF NOT EXISTS ORDERS (
            ID bigint NOT NULL UNIQUE,
    		CASHBACK double precision NOT NULL,
    		PLACED_AT text NOT NULL,
            LOGIN varchar NOT NULL
        );

-- +goose Down
DROP TABLE ORDERS;