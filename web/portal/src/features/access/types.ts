import type { MemberRole } from '../admin/types';

/**
 * Permission identifiers following the pattern: resource.action
 * Examples: org.members.read, org.budget.write, apikey.create
 */
export type Permission = 
  | 'org.read'
  | 'org.write'
  | 'org.delete'
  | 'org.members.read'
  | 'org.members.invite'
  | 'org.members.remove'
  | 'org.members.update_role'
  | 'org.budget.read'
  | 'org.budget.write'
  | 'apikey.read'
  | 'apikey.create'
  | 'apikey.rotate'
  | 'apikey.revoke'
  | 'usage.read'
  | 'audit.read';

/**
 * Role definition with permissions and metadata
 */
export interface RoleDefinition {
  role_key: MemberRole;
  display_name: string;
  description: string;
  permissions: Permission[];
  feature_flags?: string[];
}

/**
 * Permission matrix mapping roles to their allowed permissions
 */
export const PERMISSION_MATRIX: Record<MemberRole, Permission[]> = {
  owner: [
    'org.read',
    'org.write',
    'org.delete',
    'org.members.read',
    'org.members.invite',
    'org.members.remove',
    'org.members.update_role',
    'org.budget.read',
    'org.budget.write',
    'apikey.read',
    'apikey.create',
    'apikey.rotate',
    'apikey.revoke',
    'usage.read',
    'audit.read',
  ],
  admin: [
    'org.read',
    'org.write',
    'org.members.read',
    'org.members.invite',
    'org.members.remove',
    'org.members.update_role',
    'org.budget.read',
    'org.budget.write',
    'apikey.read',
    'apikey.create',
    'apikey.rotate',
    'apikey.revoke',
    'usage.read',
    'audit.read',
  ],
  manager: [
    'org.read',
    'org.members.read',
    'usage.read',
  ],
  analyst: [
    'usage.read',
  ],
  custom: [], // Custom roles have permissions defined via scopes
};

/**
 * Role metadata for display and descriptions
 */
export const ROLE_METADATA: Record<MemberRole, Omit<RoleDefinition, 'permissions'>> = {
  owner: {
    role_key: 'owner',
    display_name: 'Owner',
    description: 'Full organization control. Can manage all settings and delete organization.',
  },
  admin: {
    role_key: 'admin',
    display_name: 'Admin',
    description: 'Administrative access. Can manage members, API keys, and budgets. Cannot delete organization.',
  },
  manager: {
    role_key: 'manager',
    display_name: 'Manager',
    description: 'Team management access. Can view usage analytics and manage team members.',
  },
  analyst: {
    role_key: 'analyst',
    display_name: 'Analyst',
    description: 'Read-only analytics access. Can view usage reports but cannot modify settings.',
  },
  custom: {
    role_key: 'custom',
    display_name: 'Custom',
    description: 'Custom role with permissions defined via scopes.',
  },
};

/**
 * Check if a role has a specific permission
 */
export function hasPermission(role: MemberRole, permission: Permission, customScopes?: string[]): boolean {
  // Custom roles check scopes instead of permission matrix
  if (role === 'custom' && customScopes) {
    return customScopes.includes(permission);
  }

  const rolePermissions = PERMISSION_MATRIX[role] || [];
  return rolePermissions.includes(permission);
}

/**
 * Get all permissions for a role
 */
export function getRolePermissions(role: MemberRole, customScopes?: string[]): Permission[] {
  if (role === 'custom' && customScopes) {
    return customScopes.filter((scope): scope is Permission => 
      Object.values(PERMISSION_MATRIX).flat().includes(scope as Permission)
    ) as Permission[];
  }

  return PERMISSION_MATRIX[role] || [];
}

/**
 * Route permission mapping - which routes require which permissions
 */
export const ROUTE_PERMISSIONS: Record<string, Permission[]> = {
  '/admin/organization': ['org.read'],
  '/admin/members': ['org.members.read'],
  '/admin/budgets': ['org.budget.read'],
  '/admin/api-keys': ['apikey.read'],
  '/usage': ['usage.read'],
};


