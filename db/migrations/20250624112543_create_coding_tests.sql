-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS coding_tests (
                                            id VARCHAR(36) PRIMARY KEY, -- UUID
                                            company_id INTEGER NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
                                            problem_id INTEGER NOT NULL REFERENCES problems(id) ON DELETE CASCADE,
                                            candidate_name VARCHAR(255),
                                            candidate_email VARCHAR(255),
                                            status VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending, started, completed, expired
                                            started_at TIMESTAMP WITH TIME ZONE,
                                            completed_at TIMESTAMP WITH TIME ZONE,
                                            expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
                                            test_duration_minutes INTEGER NOT NULL DEFAULT 60,
                                            submission_code TEXT,
                                            passed_percentage INTEGER, -- 0-100
                                            created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
                                            updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_coding_tests_company_id ON coding_tests(company_id);
CREATE INDEX idx_coding_tests_status ON coding_tests(status);
CREATE INDEX idx_coding_tests_expires_at ON coding_tests(expires_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE coding_tests;
-- +goose StatementEnd