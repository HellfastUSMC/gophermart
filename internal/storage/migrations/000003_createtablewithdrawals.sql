-- +goose Up
CREATE TABLE IF NOT EXISTS WITHDRAWALS (
            ID serial NOT NULL UNIQUE PRIMARY KEY,
            ORDER_ID BIGINT NOT NULL UNIQUE,
    		SUM double precision NOT NULL,
    		PLACED_AT text NOT NULL,
            LOGIN varchar NOT NULL
        );

-- +goose Down
DROP TABLE WITHDRAWALS;