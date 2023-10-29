-- +goose Up
CREATE TABLE IF NOT EXISTS ORDERS (
            ID varchar NOT NULL UNIQUE PRIMARY KEY,
    		CASHBACK double precision NOT NULL,
    		PLACED_AT text NOT NULL,
            LOGIN varchar NOT NULL,
            STATUS varchar NOT NULL
        );

-- +goose Down
DROP TABLE ORDERS;