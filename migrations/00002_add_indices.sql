-- +goose Up
-- +goose StatementBegin
CREATE INDEX idx_review_pr_user ON review(pull_request_id, user_id);
CREATE INDEX idx_pr_status ON pull_request(status) WHERE status = 'OPEN';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
-- +goose StatementEnd
