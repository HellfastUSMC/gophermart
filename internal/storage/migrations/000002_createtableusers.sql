-- +goose Up
CREATE TABLE IF NOT EXISTS USERS (
            ID serial NOT NULL UNIQUE,
    		LOGIN varchar NOT NULL UNIQUE PRIMARY KEY,
            PASSWORD varchar NOT NULL,
    		CASHBACK double precision NOT NULL,
    		CASHBACK_ALL double precision NOT NULL,
    		WITHDRAWN double precision NOT NULL
        );

-- +goose Down
DROP TABLE USERS;