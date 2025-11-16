import { useState } from 'react';
import { Link, useLocation } from '@tanstack/react-router';
import { useAuth } from '@/providers/AuthProvider';
import { ROUTE_PERMISSIONS, hasPermission } from '@/features/access/types';
import type { MemberRole } from '@/features/admin/types';

interface NavItem {
  label: string;
  path: string;
  icon?: React.ReactNode;
}

/**
 * Role-aware navigation component
 * Only shows navigation items the user has permission to access
 */
export function RoleAwareNav() {
  const { user, isAuthenticated } = useAuth();
  const location = useLocation();

  if (!isAuthenticated || !user) {
    return null;
  }

  const navItems: NavItem[] = [
    { label: 'Organization', path: '/admin/organization' },
    { label: 'Members', path: '/admin/members' },
    { label: 'Budgets', path: '/admin/budgets' },
    { label: 'API Keys', path: '/admin/api-keys' },
    { label: 'Usage', path: '/usage' },
  ];

  // Filter nav items based on permissions
  const visibleItems = navItems.filter((item) => {
    const requiredPermissions = ROUTE_PERMISSIONS[item.path] || [];
    if (requiredPermissions.length === 0) {
      return true; // No permissions required, show to all authenticated users
    }

    // Check if user has any of the required permissions
    const userRole = user.roles[0] as MemberRole;
    return requiredPermissions.some((permission) =>
      hasPermission(userRole, permission, user.scopes)
    );
  });

  return (
    <nav className="hidden md:flex items-center space-x-4" aria-label="Main navigation">
      {visibleItems.map((item) => {
        const isActive = location.pathname === item.path || location.pathname.startsWith(item.path + '/');
        
        return (
          <Link
            key={item.path}
            to={item.path}
            className={`px-3 py-2 rounded-md text-sm font-medium focus-visible-ring ${
              isActive
                ? 'bg-primary text-white'
                : 'text-gray-700 hover:text-primary hover:bg-gray-100'
            }`}
          >
            {item.label}
          </Link>
        );
      })}
    </nav>
  );
}

/**
 * Mobile navigation menu (hamburger menu)
 */
export function MobileRoleAwareNav() {
  const [isOpen, setIsOpen] = useState(false);
  const { user, isAuthenticated } = useAuth();
  const location = useLocation();

  if (!isAuthenticated || !user) {
    return null;
  }

  const navItems: NavItem[] = [
    { label: 'Organization', path: '/admin/organization' },
    { label: 'Members', path: '/admin/members' },
    { label: 'Budgets', path: '/admin/budgets' },
    { label: 'API Keys', path: '/admin/api-keys' },
    { label: 'Usage', path: '/usage' },
  ];

  // Filter nav items (same logic as desktop nav)
  const visibleItems = navItems.filter((item) => {
    const requiredPermissions = ROUTE_PERMISSIONS[item.path] || [];
    if (requiredPermissions.length === 0) return true;

    const userRole = user.roles[0] as MemberRole;
    return requiredPermissions.some((permission) =>
      hasPermission(userRole, permission, user.scopes)
    );
  });

  return (
    <div className="md:hidden">
      <button
        onClick={() => setIsOpen(!isOpen)}
        className="p-2 rounded-md text-gray-700 hover:bg-gray-100 focus:outline-none focus:ring-2 focus:ring-primary"
        aria-label="Toggle navigation menu"
        aria-expanded={isOpen}
      >
        <svg
          className="h-6 w-6"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          {isOpen ? (
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M6 18L18 6M6 6l12 12"
            />
          ) : (
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M4 6h16M4 12h16M4 18h16"
            />
          )}
        </svg>
      </button>

      {isOpen && (
        <div className="absolute top-16 left-0 right-0 bg-white border-b border-gray-200 shadow-lg z-50">
          <nav className="px-4 py-2 space-y-1">
            {visibleItems.map((item) => {
              const isActive = location.pathname === item.path;
              return (
                <Link
                  key={item.path}
                  to={item.path}
                  onClick={() => setIsOpen(false)}
                  className={`block px-3 py-2 rounded-md text-base font-medium ${
                    isActive
                      ? 'bg-primary text-white'
                      : 'text-gray-700 hover:bg-gray-100'
                  }`}
                >
                  {item.label}
                </Link>
              );
            })}
          </nav>
        </div>
      )}
    </div>
  );
}

