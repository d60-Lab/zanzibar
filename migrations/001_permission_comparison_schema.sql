-- =====================================================
-- Permission System Comparison: Zanzibar vs MySQL
-- =====================================================
-- This migration creates the complete schema for comparing
-- traditional MySQL expanded permissions vs. Zanzibar-style
-- tuple-based permissions.
-- =====================================================

-- Disable foreign key checks during table creation
SET FOREIGN_KEY_CHECKS = 0;

-- =====================================================
-- SHARED BUSINESS TABLES
-- =====================================================

-- Users table (10,000 records expected)
CREATE TABLE IF NOT EXISTS users (
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    email VARCHAR(100) UNIQUE NOT NULL,
    primary_department_id VARCHAR(36),
    is_superuser BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Departments table (hierarchical, ~2,000 departments expected)
CREATE TABLE IF NOT EXISTS departments (
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    parent_id VARCHAR(36) NULL,
    level INT NOT NULL CHECK (level BETWEEN 1 AND 5),
    manager_id VARCHAR(36),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_parent (parent_id),
    INDEX idx_level (level),
    INDEX idx_manager (manager_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- User-Department relationships (multi-department support)
CREATE TABLE IF NOT EXISTS user_departments (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id VARCHAR(36) NOT NULL,
    department_id VARCHAR(36) NOT NULL,
    role VARCHAR(20) NOT NULL CHECK (role IN ('member', 'leader', 'director')),
    is_primary BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_user_dept (user_id, department_id),
    INDEX idx_user (user_id),
    INDEX idx_department (department_id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (department_id) REFERENCES departments(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Management relations (pre-computed management paths)
CREATE TABLE IF NOT EXISTS management_relations (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    manager_user_id VARCHAR(36) NOT NULL,
    subordinate_user_id VARCHAR(36) NOT NULL,
    department_id VARCHAR(36) NOT NULL,
    management_level INT NOT NULL CHECK (management_level BETWEEN 1 AND 5),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_manager_subordinate_dept (manager_user_id, subordinate_user_id, department_id),
    INDEX idx_manager (manager_user_id),
    INDEX idx_subordinate (subordinate_user_id),
    INDEX idx_dept (department_id),
    FOREIGN KEY (manager_user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (subordinate_user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (department_id) REFERENCES departments(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Customers table (100,000 records expected)
CREATE TABLE IF NOT EXISTS customers (
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Documents table (~500,000 records expected)
CREATE TABLE IF NOT EXISTS documents (
    id VARCHAR(36) PRIMARY KEY,
    title VARCHAR(200) NOT NULL,
    customer_id VARCHAR(36) NOT NULL,
    creator_id VARCHAR(36) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL,
    INDEX idx_customer (customer_id),
    INDEX idx_creator (creator_id),
    FOREIGN KEY (customer_id) REFERENCES customers(id) ON DELETE CASCADE,
    FOREIGN KEY (creator_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Customer followers (1-10 followers per customer)
CREATE TABLE IF NOT EXISTS customer_followers (
    customer_id VARCHAR(36) NOT NULL,
    user_id VARCHAR(36) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (customer_id, user_id),
    INDEX idx_user (user_id),
    FOREIGN KEY (customer_id) REFERENCES customers(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- =====================================================
-- MYSQL-ONLY: EXPANDED PERMISSION TABLE
-- =====================================================
-- This table stores pre-computed, denormalized permissions.
-- Expected size: 10-20 million rows.
-- =====================================================

CREATE TABLE IF NOT EXISTS document_permissions_mysql (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id VARCHAR(36) NOT NULL,
    document_id VARCHAR(36) NOT NULL,
    permission_type ENUM('viewer', 'editor', 'owner') NOT NULL,
    source_type ENUM('direct', 'customer_follower', 'manager_chain', 'superuser') NOT NULL,
    source_id VARCHAR(36) NULL, -- For tracing permission origin
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    -- Critical indexes for MySQL query performance
    UNIQUE KEY uk_user_doc (user_id, document_id, permission_type),
    INDEX idx_user (user_id),
    INDEX idx_document (document_id),
    INDEX idx_source (source_type, source_id),
    INDEX idx_user_permission (user_id, permission_type),

    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (document_id) REFERENCES documents(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- =====================================================
-- ZANZIBAR-STYLE: RELATION TUPLES
-- =====================================================
-- This table stores base relationship tuples only.
-- Expected size: ~1 million tuples.
-- Permissions are computed by graph traversal.
-- =====================================================

CREATE TABLE IF NOT EXISTS relation_tuples (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,

    -- Object: what we're protecting
    namespace VARCHAR(50) NOT NULL,
    object_id VARCHAR(36) NOT NULL,
    relation VARCHAR(50) NOT NULL,

    -- Subject: who has access
    subject_namespace VARCHAR(50) NOT NULL,
    subject_id VARCHAR(36) NOT NULL,

    -- For computed/union relations (advanced Zanzibar feature)
    userset_namespace VARCHAR(50) NULL,
    userset_relation VARCHAR(50) NULL,

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    -- Critical indexes for Zanzibar queries
    UNIQUE KEY uk_tuple (namespace, object_id, relation, subject_namespace, subject_id),
    INDEX idx_object (namespace, object_id, relation),
    INDEX idx_subject (subject_namespace, subject_id, relation),
    INDEX idx_computed (userset_namespace, userset_relation),
    INDEX idx_namespace_object (namespace, object_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- =====================================================
-- INDEXES FOR PERFORMANCE
-- =====================================================

-- Composite index for common permission check queries
CREATE INDEX idx_mysql_user_doc_type ON document_permissions_mysql(user_id, permission_type, document_id);

-- Index for finding user's documents quickly
CREATE INDEX idx_mysql_user_docs ON document_permissions_mysql(user_id, document_id);

-- Zanzibar index for finding all tuples for an object
CREATE INDEX idx_zanzibar_object_lookup ON relation_tuples(namespace, object_id, relation, subject_namespace);

-- Zanzibar index for finding all tuples for a subject
CREATE INDEX idx_zanzibar_subject_lookup ON relation_tuples(subject_namespace, subject_id, namespace);

-- =====================================================
-- PERFORMANCE MONITORING TABLES
-- =====================================================

-- Query performance logs
CREATE TABLE IF NOT EXISTS benchmark_logs (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    test_name VARCHAR(100) NOT NULL,
    engine_type ENUM('mysql', 'zanzibar') NOT NULL,
    operation_type VARCHAR(50) NOT NULL,
    duration_ms DECIMAL(10, 3) NOT NULL,
    rows_affected INT DEFAULT 0,
    cache_hit BOOLEAN DEFAULT FALSE,
    error_message TEXT NULL,
    metadata JSON NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_test_name (test_name),
    INDEX idx_engine (engine_type),
    INDEX idx_operation (operation_type),
    INDEX idx_created (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- System resource usage during benchmarks
CREATE TABLE IF NOT EXISTS benchmark_metrics (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    benchmark_id BIGINT NOT NULL,
    metric_type VARCHAR(50) NOT NULL,
    metric_name VARCHAR(100) NOT NULL,
    metric_value DECIMAL(15, 3) NOT NULL,
    unit VARCHAR(20) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_benchmark_id (benchmark_id),
    INDEX idx_metric_type (metric_type),
    FOREIGN KEY (benchmark_id) REFERENCES benchmark_logs(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- =====================================================
-- VIEWS FOR EASY DATA INSPECTION
-- =====================================================

-- View: MySQL permission summary
CREATE OR REPLACE VIEW v_mysql_permission_stats AS
SELECT
    source_type,
    permission_type,
    COUNT(*) as total_permissions,
    COUNT(DISTINCT user_id) as unique_users,
    COUNT(DISTINCT document_id) as unique_documents
FROM document_permissions_mysql
GROUP BY source_type, permission_type;

-- View: Zanzibar tuple summary
CREATE OR REPLACE VIEW v_zanzibar_tuple_stats AS
SELECT
    namespace,
    relation,
    COUNT(*) as total_tuples,
    COUNT(DISTINCT object_id) as unique_objects,
    COUNT(DISTINCT subject_id) as unique_subjects
FROM relation_tuples
GROUP BY namespace, relation;

-- View: Storage comparison
CREATE OR REPLACE VIEW v_storage_comparison AS
SELECT
    'MySQL' as engine_type,
    'document_permissions_mysql' as table_name,
    TABLE_ROWS as row_count,
    ROUND(DATA_LENGTH / 1024 / 1024, 2) as data_size_mb,
    ROUND(INDEX_LENGTH / 1024 / 1024, 2) as index_size_mb,
    ROUND((DATA_LENGTH + INDEX_LENGTH) / 1024 / 1024, 2) as total_size_mb
FROM information_schema.TABLES
WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'document_permissions_mysql'
UNION ALL
SELECT
    'Zanzibar' as engine_type,
    'relation_tuples' as table_name,
    TABLE_ROWS as row_count,
    ROUND(DATA_LENGTH / 1024 / 1024, 2) as data_size_mb,
    ROUND(INDEX_LENGTH / 1024 / 1024, 2) as index_size_mb,
    ROUND((DATA_LENGTH + INDEX_LENGTH) / 1024 / 1024, 2) as total_size_mb
FROM information_schema.TABLES
WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'relation_tuples';

-- =====================================================
-- SAMPLE DATA INSERTION (For testing)
-- =====================================================

-- Note: Test data will be generated by the data generator tool
-- These are just examples for manual testing

-- Example departments (5-level hierarchy)
-- INSERT INTO departments (id, name, parent_id, level) VALUES
-- ('dept-1', 'Headquarters', NULL, 1),
-- ('dept-2', 'Engineering', 'dept-1', 2),
-- ('dept-3', 'Sales', 'dept-1', 2),
-- ('dept-4', 'Backend Team', 'dept-2', 3),
-- ('dept-5', 'Frontend Team', 'dept-2', 3);

-- Example users
-- INSERT INTO users (id, name, email, primary_department_id) VALUES
-- ('user-1', 'Admin User', 'admin@example.com', 'dept-1'),
-- ('user-2', 'Engineer User', 'engineer@example.com', 'dept-4'),
-- ('user-3', 'Sales User', 'sales@example.com', 'dept-3');

-- Example documents
-- INSERT INTO customers (id, name) VALUES
-- ('customer-1', 'Acme Corp');

-- INSERT INTO documents (id, title, customer_id, creator_id) VALUES
-- ('doc-1', 'Project Plan', 'customer-1', 'user-2');

-- Example Zanzibar tuples
-- INSERT INTO relation_tuples (namespace, object_id, relation, subject_namespace, subject_id) VALUES
-- ('system', 'root', 'admin', 'user', 'user-1'),
-- ('document', 'doc-1', 'viewer', 'user', 'user-2'),
-- ('document', 'doc-1', 'owner_customer', 'customer', 'customer-1'),
-- ('customer', 'customer-1', 'follower', 'user', 'user-3');

-- =====================================================
-- DOCUMENT READ TRACKING
-- =====================================================

-- Document read status table
CREATE TABLE IF NOT EXISTS document_reads (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id VARCHAR(36) NOT NULL,
    document_id VARCHAR(36) NOT NULL,
    read_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_user_doc (user_id, document_id),
    INDEX idx_user (user_id),
    INDEX idx_document (document_id),
    INDEX idx_read_at (read_at),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (document_id) REFERENCES documents(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- =====================================================
-- END OF MIGRATION
-- =====================================================

-- Re-enable foreign key checks
SET FOREIGN_KEY_CHECKS = 1;
