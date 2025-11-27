# Seeded Test Users

This document contains the credentials for test users seeded in the database.

## System Admin

- **Email**: `sys-admin@example.com`
- **Password**: `SysAdmin2024!SecurePass`
- **Role**: System Admin (stored in metadata)
- **Purpose**: System-level administration

## Acme Ltd Organization

**Organization Name**: Acme Ltd  
**Slug**: `acme-ltd`

### Org Admin User

- **Email**: `admin@example-acme.com`
- **Password**: `AcmeAdmin2024!Secure`
- **Role**: `admin`
- **Purpose**: Primary organization administrator for Acme Ltd
- **Use Case**: Tests that require a user account should use this user

### Manager

- **Email**: `manager@example-acme.com`
- **Password**: `AcmeManager2024!Secure`
- **Role**: `manager`
- **Purpose**: Team management access for Acme Ltd

## JoeBlogs Ltd Organization

**Organization Name**: JoeBlogs Ltd  
**Slug**: `joeblogs-ltd`

### Org Admin User

- **Email**: `admin@example-joeblogs.com`
- **Password**: `JoeBlogsAdmin2024!Secure`
- **Role**: `admin`
- **Purpose**: Primary organization administrator for JoeBlogs Ltd
- **Use Case**: Tests that check one org cannot access another should use this user

### Manager

- **Email**: `manager@example-joeblogs.com`
- **Password**: `JoeBlogsManager2024!Secure`
- **Role**: `manager`
- **Purpose**: Team management access for JoeBlogs Ltd

## Usage in Tests

- **Acme Ltd users** (`admin@example-acme.com`, `manager@example-acme.com`): Use for general tests that require a user account
- **JoeBlogs Ltd users** (`admin@example-joeblogs.com`, `manager@example-joeblogs.com`): Use for tests that verify organization isolation and access control

## Notes

- All passwords follow the pattern: `[Org/User]2024!Secure[Optional]`
- Roles are stored in user metadata as `{"roles": ["role_name"]}`
- All users are created with `status: "active"`
- Organizations are created with `status: "active"`

