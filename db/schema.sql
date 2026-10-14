-- URL shortener schema
CREATE TABLE IF NOT EXISTS links (
    id         BIGINT AUTO_INCREMENT PRIMARY KEY,
    code       VARCHAR(32)   NOT NULL UNIQUE,
    target_url VARCHAR(2048) NOT NULL,
    clicks     BIGINT        NOT NULL DEFAULT 0,
    created_at TIMESTAMP     NOT NULL DEFAULT CURRENT_TIMESTAMP
);
