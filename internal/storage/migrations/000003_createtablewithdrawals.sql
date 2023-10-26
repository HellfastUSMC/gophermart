-- +goose Up
CREATE TABLE IF NOT EXISTS WITHDRAWALS (
            ID serial NOT NULL UNIQUE,
            ORDER_ID BIGINT NOT NULL,
    		SUM double precision NOT NULL,
    		PLACED_AT text NOT NULL,
            LOGIN varchar NOT NULL PRIMARY KEY
        );

-- +goose Down
DROP TABLE WITHDRAWALS;