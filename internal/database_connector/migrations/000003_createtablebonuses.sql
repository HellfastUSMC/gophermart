-- +goose Up
CREATE TABLE IF NOT EXISTS BONUSES (
            ID serial NOT NULL UNIQUE PRIMARY KEY,
            ORDER_ID varchar NOT NULL UNIQUE,
    		SUM double precision NOT NULL,
    		PLACED_AT text NOT NULL,
            LOGIN varchar NOT NULL,
            SUB bool NOT NULL
        );

-- +goose Down
DROP TABLE BONUSES;