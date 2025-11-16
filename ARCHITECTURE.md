# Architecture

This document provides a high-level overview of the AI-as-a-Service platform architecture.

## System Overview

The platform is designed as a set of cooperating microservices. The central component is the **API Router Service**, which acts as the public-facing gateway for all AI model inference requests. It is responsible for authentication, authorization, rate limiting, budget enforcement, and routing requests to the appropriate backend model services.

Other key services include:

*   **User & Organization Service**: Manages users, organizations, and API keys.
*   **Budget Service**: Tracks and enforces spending budgets for organizations.
*   **Analytics Service**: Collects and processes usage data for billing and analysis.
*   **Admin CLI**: A command-line tool for administering the platform.

The system also relies on several backing services:

*   **Redis**: Used for caching and rate limiting.
*   **Kafka**: Used for asynchronous messaging, particularly for usage data.
*   **PostgreSQL**: The primary database for services like the User & Organization Service.

## Service Interaction Diagram

The following diagram illustrates the primary request flow and service interactions:

```
+-----------------+      +----------------------+      +--------------------+
|                 |----->| User & Org Service   |----->|      Database      |
| API Key Auth    |      | (Authentication)     |      |     (PostgreSQL)   |
+-----------------+      +----------------------+      +--------------------+
       ^
       |
+------+----------+      +----------------------+
|                 |----->|    Budget Service    |
| API Router      |      | (Budget Enforcement) |
| (Gateway)       |      +----------------------+
|                 |
+------+-+-+------+      +----------------------+
       | | |             |   Analytics Service  |
       | | +------------>| (Usage Tracking)     |
       | |               +----------------------+
       | |                      ^
       | |                      |
       | +----------------------V--------------------+
       |                        |                     |
       |                      +---+                   |
       +--------------------->|   |                   |
                              |AI |<------------------+
       +--------------------->|   |
       |                      |   |
       |                      +---+
       |                        |
       +--------------------->|...| (etc.)
                              +---+
                         Backend Model
                            Services
```

## Services

### API Router Service (`api-router-service`)

*   **Description**: The main entry point for all API requests. It handles routing, authentication, rate limiting, and more.
*   **Language**: Go
*   **Dependencies**: User & Org Service, Budget Service, Redis, Kafka.

### User & Organization Service (`user-org-service`)

*   **Description**: Manages all data related to users, organizations, API keys, and authentication.
*   **Language**: Go
*   **Dependencies**: PostgreSQL.

### Budget Service (`budget-service`)

*   **Description**: Manages and enforces spending limits for organizations.
*   **Language**: Go
*   **Dependencies**: (Likely PostgreSQL or another database).

### Analytics Service (`analytics-service`)

*   **Description**: Consumes usage data from Kafka to provide analytics and billing information.
*   **Language**: (Likely Go or Python)
*   **Dependencies**: Kafka, (Likely a data warehouse like ClickHouse or Snowflake).

### Admin CLI (`admin-cli`)

*   **Description**: A command-line interface for administrators to manage the platform (e.g., creating users, managing organizations).
*   **Language**: Go
*   **Dependencies**: Interacts with the APIs of the various services.

### Hello Service & World Service

*   **Description**: These are likely example or template services, demonstrating how to build a new service within the platform's architecture.
*   **Language**: Go

## Next Steps

For more detailed information on a specific service, please refer to the `README.md` file within that service's directory (e.g., `services/api-router-service/README.md`).
