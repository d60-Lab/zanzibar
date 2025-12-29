# Zanzibar vs MySQL Permission System: Performance Comparison Design

**Date**: 2025-01-29
**Author**: Research Team
**Status**: Design Approved

## Executive Summary

This document outlines a comprehensive technical research project to compare two permission system architectures: a traditional denormalized MySQL approach and a Zanzibar-inspired tuple-based graph approach. The goal is to produce empirical data demonstrating the performance, storage, and maintenance differences between these approaches.

The research will support a technical article analyzing Google Zanzibar's applicability to enterprise document permission systems.

## Background

### Problem Statement

Many companies face the "permission explosion" problem when implementing document access controls:

- **Original requirements**: Simple hierarchical permissions (managers see subordinates' documents)
- **Implementation**: Denormalized permission tables with pre-computed relationships
- **Result**: 10+ million permission rows, complex maintenance, slow updates

Key pain points:
1. Department manager changes require rebuilding millions of permission rows
2. Employee department transfers trigger cascading updates
3. Customer team changes affect thousands of documents
4. Data consistency delays due to background jobs

### Research Hypothesis

**Zanzibar's tuple-based approach with real-time graph resolution provides:**
- 90% reduction in storage
- 100-1000x faster maintenance operations
- Competitive or better read performance
- Immediate data consistency

## System Architecture

### Overview

The project implements two independent permission engines sharing the same test data:

```
┌─────────────────────────────────────────────────────┐
│              Test Data Generator                     │
│  (100K customers, 10K users, 500K documents)        │
└───────────────────┬─────────────────────────────────┘
                    │
        ┌───────────┴───────────┐
        ▼                       ▼
┌──────────────┐        ┌──────────────┐
│  MySQL       │        │  Zanzibar    │
│  Approach    │        │  Approach    │
│              │        │              │
│ Expanded     │        │ Tuple-based  │
│ Permission   │        │ Graph Engine │
│ Table        │        │              │
│ (10M rows)   │        │ (~1M tuples) │
└──────┬───────┘        └──────┬───────┘
       │                       │
       └───────────┬───────────┘
                   ▼
          ┌────────────────┐
          │ Benchmark      │
          │ Suite          │
          │                │
          │ Performance    │
          │ Comparison     │
          └────────────────┘
```

### Component Specifications

**1. MySQL Approach (Baseline)**
- Pre-computed `document_permissions_mysql` table
- Denormalized storage: `(user_id, document_id, permission_type, source_type, source_id)`
- Query strategy: Direct SELECT with WHERE clauses
- Maintenance: Background jobs to recompute permissions

**2. Zanzibar Approach**
- `relation_tuples` table storing 5 core relationship types
- In-memory graph engine with recursive traversal
- LRU cache for hot permission checks
- Maintenance: Direct tuple updates, immediate consistency

**3. Shared Infrastructure**
- Identical business tables (users, departments, customers, documents)
- Same test data for fair comparison
- Benchmark framework with metrics collection
- API endpoints for both systems

## Database Schema

### Shared Business Tables

#### Users (10,000 records)
```sql
CREATE TABLE users (
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(100),
    email VARCHAR(100) UNIQUE,
    primary_department_id VARCHAR(36),
    is_superuser BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP
);
```

#### Departments (hierarchical, ~2,000 departments)
```sql
CREATE TABLE departments (
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(100),
    parent_id VARCHAR(36) NULL,
    level INT, -- 1-5 levels deep
    manager_id VARCHAR(36),
    INDEX idx_parent (parent_id),
    INDEX idx_level (level),
    INDEX idx_manager (manager_id)
);
```

#### User-Department Relationships (Multi-Department Support)
```sql
CREATE TABLE user_departments (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id VARCHAR(36),
    department_id VARCHAR(36),
    role VARCHAR(20), -- 'member', 'leader', 'director'
    is_primary BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP,
    UNIQUE KEY uk_user_dept (user_id, department_id),
    INDEX idx_user (user_id),
    INDEX idx_department (department_id)
);
```

#### Management Relations
```sql
CREATE TABLE management_relations (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    manager_user_id VARCHAR(36),
    subordinate_user_id VARCHAR(36),
    department_id VARCHAR(36),
    management_level INT,
    created_at TIMESTAMP,
    UNIQUE KEY uk_manager_subordinate_dept (manager_user_id, subordinate_user_id, department_id),
    INDEX idx_manager (manager_user_id),
    INDEX idx_subordinate (subordinate_user_id)
);
```

#### Customers (100,000 records)
```sql
CREATE TABLE customers (
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(100),
    created_at TIMESTAMP
);
```

#### Documents (~500,000 records)
```sql
CREATE TABLE documents (
    id VARCHAR(36) PRIMARY KEY,
    title VARCHAR(200),
    customer_id VARCHAR(36),
    creator_id VARCHAR(36),
    created_at TIMESTAMP,
    INDEX idx_customer (customer_id),
    INDEX idx_creator (creator_id)
);
```

#### Customer Followers
```sql
CREATE TABLE customer_followers (
    customer_id VARCHAR(36),
    user_id VARCHAR(36),
    PRIMARY KEY (customer_id, user_id),
    INDEX idx_user (user_id)
);
```

### MySQL-Only: Expanded Permission Table

```sql
CREATE TABLE document_permissions_mysql (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id VARCHAR(36),
    document_id VARCHAR(36),
    permission_type ENUM('viewer', 'editor', 'owner'),
    source_type ENUM('direct', 'customer_follower', 'manager_chain', 'superuser'),
    source_id VARCHAR(36) NULL,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    UNIQUE KEY uk_user_doc (user_id, document_id, permission_type),
    INDEX idx_user (user_id),
    INDEX idx_document (document_id),
    INDEX idx_source (source_type, source_id)
);
```

**Expected size**: 10-20 million rows

### Zanzibar-Only: Relation Tuples

```sql
CREATE TABLE relation_tuples (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    namespace VARCHAR(50),
    object_id VARCHAR(36),
    relation VARCHAR(50),
    subject_namespace VARCHAR(50),
    subject_id VARCHAR(36),
    userset_namespace VARCHAR(50) NULL,
    userset_relation VARCHAR(50) NULL,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,

    UNIQUE KEY uk_tuple (namespace, object_id, relation, subject_namespace, subject_id),
    INDEX idx_object (namespace, object_id, relation),
    INDEX idx_subject (subject_namespace, subject_id, relation),
    INDEX idx_computed (userset_namespace, userset_relation)
);
```

**Expected size**: ~1 million tuples

#### Core Tuple Types

1. **Direct document access**: `document:doc-123#viewer@user:user-456`
2. **Document ownership**: `document:doc-123#owner_customer@customer:cust-789`
3. **Customer followers**: `customer:cust-789#follower@user:user-456`
4. **Department membership**: `department:dept-1#member@user:user-A`
5. **Department managers**: `department:dept-1#manager@user:manager-B`
6. **Superuser**: `system:root#admin@user:admin-001`

## Data Generation Strategy

### Realistic Distributions

#### Organization Structure
- **10,000 users** across ~2,000 departments (5 levels)
- **Multi-department affiliation**:
  - 80% employees: 1 department
  - 15% employees: 2 departments
  - 4% employees: 3 departments
  - 1% employees: 4-5 departments (cross-functional roles)
- **Management hierarchy**: Max 5 levels, no circular dependencies

#### Customer Distribution
- **100,000 customers** with realistic follower distribution:
  - 30%: 1 follower
  - 40%: 2-3 followers
  - 20%: 4-6 followers
  - 10%: 7-10 followers (key accounts)

#### Document Distribution (Zipfian)
- **~500,000 documents** per customer:
  - 70% customers: 1-5 documents
  - 20% customers: 6-20 documents
  - 8% customers: 21-100 documents
  - 2% customers: 100-500 documents (enterprise accounts)
- **Access patterns** (80/20 rule):
  - 80% of access focused on 20% of documents
  - Cold documents: 5-10 people with access
  - Hot documents: 50-200 people with access

#### Permission Source Distribution
In MySQL expanded table:
- Direct: 15%
- Customer Follower: 45%
- Manager Chain: 39%
- Superuser: 1%

### Generation Algorithm

**Phase 1: Organization Hierarchy**
1. Build 5-level department tree
2. Assign managers to departments
3. Distribute users to departments (1-5 per user)
4. Build `management_relations` table (all management paths)

**Phase 2: Customers & Followers**
1. Create 100,000 customers
2. Assign 1-10 followers per customer (weighted distribution)

**Phase 3: Documents**
1. Generate documents per customer (zipf distribution)
2. Assign creator from customer's followers
3. Randomly assign direct viewers (5-10% of docs)

**Phase 4: Build MySQL Permissions** (Expensive)
For each document:
1. Add direct viewers → permissions table
2. Add customer followers → permissions table
3. For each person with permission, recursively expand manager chain (up to 5 levels)
4. Add all superusers

**Phase 5: Build Zanzibar Tuples** (Simple)
1. Insert direct document viewer tuples
2. Insert document→customer owner tuples
3. Insert customer→follower tuples
4. Insert user→department membership tuples
5. Insert department→manager tuples
6. Insert superuser tuples

### Validation

- No circular management relationships
- All documents have valid customer references
- All followers exist in users table
- Spot-check: 100 random users have identical permissions in both systems
- Verify tuple count vs. expanded row count

## Performance Testing Methodology

### Test Categories

#### 1. Read Performance

**Test A: Single Permission Check**
- Query: "Can user X access document Y?"
- Sample: 10,000 random user-document pairs
- Metrics: p50/p95/p99 latency, throughput, cache hit rate

**Test B: Batch Permission Check**
- Query: "Which of these 50 documents can user X access?"
- Sample: 1,000 batch requests
- Metrics: Total time, per-document latency

**Test C: User Document List** (Search scenario)
- Query: "Show all documents user X can access (paginated)"
- Sample: 1,000 different users
- Metrics: Query time, result accuracy, pagination performance

#### 2. Write Performance (Maintenance)

**Test D: Single Relationship Change**
Operations:
1. Change department manager (affects 100+ employees)
2. Employee changes department (affects manager chain)
3. Add/remove customer follower (affects all customer docs)
4. Grant direct document access

Metrics: Operation time, rows affected, lock wait time

**Test E: Batch Operations**
- Scenario: Move 100 employees to new department
- MySQL: Rebuild all affected permissions (background job)
- Zanzibar: Update 100 tuples
- Metrics: Total time, DB I/O, memory, consistency window

#### 3. Scalability Tests

**Test F: Concurrent Load**
- 100 concurrent users checking permissions
- Duration: 5 minutes
- Metrics: System throughput, resource usage, error rate

**Test G: Data Volume Impact**
Test at different scales:
- Baseline: 10K users, 100K customers, 500K docs
- 5x: 50K users, 500K customers, 2.5M docs
- 10x: 100K users, 1M customers, 5M docs

#### 4. Real-World Scenarios

**Test H: Organizational Restructuring**
- Scenario: Merge 3 departments into 1
- Steps: Update hierarchy, change 3 managers, move 500 employees
- MySQL: Full permission rebuild
- Zanzibar: Update ~503 tuples

**Test I: Customer Team Changes**
- Scenario: 10 large accounts get new sales teams
- Update ~75 customer followers
- Affects ~5,000 documents
- Metrics: Time until new team has access

### Benchmark Framework

```go
type BenchmarkSuite struct {
    DB                *sql.DB
    MySQLEngine       *MySQLPermissionEngine
    ZanzibarEngine    *ZanzibarPermissionEngine
    TestDataGenerator *TestDataGenerator
    MetricsRecorder   *MetricsRecorder
    Concurrency       int
    WarmupRounds      int
    TestRounds        int
}

type MetricsRecorder struct {
    LatencyHistogram *histogram.Histogram
    QueryCounter     prometheus.Counter
    ErrorCounter     prometheus.Counter
    DBStats          DBStatistics
    ResourceUsage    ResourceMetrics
}
```

### Test Execution Flow

1. Generate test data (shared by both systems)
2. For each test category (A-I):
   - Warmup: 100 queries to populate caches
   - Measure: Run tests with metrics collection
   - Verify: Spot-check 10 results for correctness
   - Clear caches (for fair comparison)
   - Repeat for both engines
3. Collect results:
   - CSV files for analysis
   - Performance comparison report
   - Charts: latency distributions, throughput curves

### Key Performance Indicators

**Storage Efficiency**
- MySQL: 10M+ rows (~2GB+)
- Zanzibar: 1M tuples (~200MB)
- Target: 90% reduction

**Read Performance**
- Cold cache: MySQL vs. Zanzibar
- Warm cache: MySQL (indexes) vs. Zanzibar (LRU)
- Target: Competitive or better

**Write Performance**
- Department manager change:
  - MySQL: 10-60 seconds (rebuild)
  - Zanzibar: <100ms (single UPDATE)
- Target: 100-1000x improvement

**Consistency**
- MySQL: Delayed (background job)
- Zanzibar: Immediate
- Target: Zero delay

## Implementation Plan

### Phase 1: Foundation (Week 1)
- [ ] Set up project structure
- [ ] Implement database migrations
- [ ] Create base domain models

### Phase 2: MySQL Engine (Week 2)
- [ ] Implement data generator
- [ ] Build permission expansion logic
- [ ] Create MySQL permission engine
- [ ] Implement permission check APIs

### Phase 3: Zanzibar Engine (Week 3)
- [ ] Implement tuple storage layer
- [ ] Build graph traversal engine
- [ ] Add LRU caching
- [ ] Create permission check APIs

### Phase 4: Benchmark Suite (Week 4)
- [ ] Implement benchmark framework
- [ ] Create test scenarios (A-I)
- [ ] Build metrics collection
- [ ] Add reporting and visualization

### Phase 5: Testing & Analysis (Week 5)
- [ ] Run full benchmark suite
- [ ] Validate data consistency
- [ ] Analyze results
- [ ] Generate technical report

## Success Criteria

1. **Functional Correctness**
   - ✅ Both systems return identical permission results (verified by sampling)
   - ✅ All permission rules correctly implemented
   - ✅ Multi-department support working

2. **Performance Targets**
   - ✅ Zanzibar storage reduction >80%
   - ✅ Maintenance operations 100-1000x faster
   - ✅ Read performance competitive (within 2x)

3. **Data Quality**
   - ✅ Realistic data distributions
   - ✅ No circular dependencies
   - ✅ All foreign keys valid

4. **Documentation**
   - ✅ Complete design document
   - ✅ Performance comparison report
   - ✅ CSDN article draft

## Deliverables

1. **Source Code**: Complete implementation of both engines
2. **Test Data**: Generated datasets for reproducibility
3. **Benchmark Results**: Raw metrics and visualizations
4. **Technical Report**: Comprehensive analysis (10-15 pages)
5. **CSDN Article**: Blog post summarizing findings (2000-3000 words)

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Multi-department logic complexity | High | Start with single-department, add complexity incrementally |
| Graph traversal performance | Medium | Implement aggressive caching and query optimization |
| Test data generation time | Medium | Use efficient batch inserts and parallel processing |
| Result validation effort | Medium | Automated spot-checking with statistical sampling |

## References

- Google Zanzibar Paper: https://arxiv.org/abs/1811.02570
- Zanzibar ACL Model: https://youtu.be/2BmHeYz6zsQ
- PostgreSQL Recursive CTEs: https://www.postgresql.org/docs/current/queries-with.html

## Appendix: Terminology

- **Tuple**: `(namespace, object_id, relation, subject_namespace, subject_id)` - Zanzibar's base unit
- **Expanded storage**: Denormalized permission tables (MySQL approach)
- **Graph traversal**: Recursive permission resolution (Zanzibar approach)
- **Management chain**: Hierarchical manager relationships
- **Zipfian distribution**: Power law distribution (80/20 rule)
