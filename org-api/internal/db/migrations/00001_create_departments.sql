-- +goose Up
CREATE TABLE departments (
    id         BIGSERIAL PRIMARY KEY,
    name       VARCHAR(200) NOT NULL,
    parent_id  BIGINT REFERENCES departments(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- unique name within same parent (non-null parent_id)
CREATE UNIQUE INDEX idx_dept_name_parent_notnull
    ON departments (name, parent_id)
    WHERE parent_id IS NOT NULL;

-- unique name among root departments (null parent_id)
CREATE UNIQUE INDEX idx_dept_name_parent_null
    ON departments (name)
    WHERE parent_id IS NULL;

-- +goose Down
DROP TABLE IF EXISTS departments;
