import { useState } from 'react';
import { Link, useLocation } from '@tanstack/react-router';
import { useAuth } from '@/providers/AuthProvider';

interface NavItem {
  label: string;
  href: string;
  icon: React.ReactNode;
  roles?: string[];
}

interface NavSection {
  title: string;
  items: NavItem[];
  defaultOpen?: boolean;
}

// Icon components (inline SVGs for simplicity)
const Icons = {
  Home: () => (
    <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6" />
    </svg>
  ),
  Key: () => (
    <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z" />
    </svg>
  ),
  Users: () => (
    <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z" />
    </svg>
  ),
  Building: () => (
    <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4" />
    </svg>
  ),
  Chart: () => (
    <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
    </svg>
  ),
  Shield: () => (
    <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
    </svg>
  ),
  Support: () => (
    <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M18.364 5.636l-3.536 3.536m0 5.656l3.536 3.536M9.172 9.172L5.636 5.636m3.536 9.192l-3.536 3.536M21 12a9 9 0 11-18 0 9 9 0 0118 0zm-5 0a4 4 0 11-8 0 4 4 0 018 0z" />
    </svg>
  ),
  ChevronDown: () => (
    <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
    </svg>
  ),
  Cube: () => (
    <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4" />
    </svg>
  ),
  Server: () => (
    <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01" />
    </svg>
  ),
  Cloud: () => (
    <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M3 15a4 4 0 004 4h9a5 5 0 10-.1-9.999 5.002 5.002 0 10-9.78 2.096A4.001 4.001 0 003 15z" />
    </svg>
  ),
  Cog: () => (
    <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
    </svg>
  ),
  Database: () => (
    <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4m0 5c0 2.21-3.582 4-8 4s-8-1.79-8-4" />
    </svg>
  ),
  GitBranch: () => (
    <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M6 3v12m0 0a3 3 0 103 3H15a3 3 0 10-3-3H9a3 3 0 01-3 3zm12-3V9a3 3 0 00-3-3H9" />
    </svg>
  ),
  Terminal: () => (
    <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
    </svg>
  ),
};

const navSections: NavSection[] = [
  {
    title: 'Dashboard',
    defaultOpen: true,
    items: [
      { label: 'Home', href: '/', icon: <Icons.Home /> },
      { label: 'Platform Status', href: '/platform/status', icon: <Icons.Server /> },
    ],
  },
  {
    title: 'Model Management',
    defaultOpen: true,
    items: [
      { label: 'Registry', href: '/models/registry', icon: <Icons.Cube /> },
      { label: 'Cache', href: '/models/cache', icon: <Icons.Database /> },
      { label: 'Deployments', href: '/models/deployments', icon: <Icons.Cloud /> },
      { label: 'Troubleshoot', href: '/models/troubleshoot', icon: <Icons.Support /> },
      { label: 'Versions', href: '/models/versions', icon: <Icons.GitBranch /> },
      { label: 'Library', href: '/models/library', icon: <Icons.Cube /> },
      { label: 'Inference', href: '/models/inference', icon: <Icons.Terminal /> },
    ],
  },
  {
    title: 'Access Control',
    defaultOpen: true,
    items: [
      { label: 'Organizations', href: '/access/organizations', icon: <Icons.Building />, roles: ['admin'] },
      { label: 'Users', href: '/access/users', icon: <Icons.Users />, roles: ['admin'] },
      { label: 'Model Access', href: '/access/user-model-access', icon: <Icons.Shield />, roles: ['admin'] },
      { label: 'API Keys', href: '/admin/api-keys', icon: <Icons.Key /> },
      { label: 'Credentials', href: '/access/credentials', icon: <Icons.Shield />, roles: ['admin'] },
    ],
  },
  {
    title: 'Platform Operations',
    defaultOpen: false,
    items: [
      { label: 'GitOps', href: '/platform/gitops', icon: <Icons.GitBranch />, roles: ['admin'] },
      { label: 'Configuration', href: '/platform/config', icon: <Icons.Cog />, roles: ['admin'] },
    ],
  },
  {
    title: 'Monitoring',
    defaultOpen: true,
    items: [
      { label: 'Usage', href: '/usage', icon: <Icons.Chart /> },
      { label: 'Audit Log', href: '/admin/audit', icon: <Icons.Shield /> },
    ],
  },
  {
    title: 'Support',
    defaultOpen: false,
    items: [
      { label: 'Console', href: '/support/console', icon: <Icons.Support />, roles: ['support', 'admin'] },
    ],
  },
];

interface SidebarSectionProps {
  section: NavSection;
  isActive: (href: string) => boolean;
}

function SidebarSection({ section, isActive }: SidebarSectionProps) {
  const [isOpen, setIsOpen] = useState(section.defaultOpen ?? true);
  const { user } = useAuth();

  // Filter items based on user roles
  const visibleItems = section.items.filter(item => {
    if (!item.roles) return true;
    return item.roles.some(role => user?.roles?.includes(role));
  });

  if (visibleItems.length === 0) return null;

  return (
    <div className="mb-2">
      <button
        onClick={() => setIsOpen(!isOpen)}
        className="w-full flex items-center justify-between px-3 py-2 text-xs font-semibold text-gray-400 uppercase tracking-wider hover:text-gray-300"
      >
        {section.title}
        <span className={`transform transition-transform ${isOpen ? 'rotate-180' : ''}`}>
          <Icons.ChevronDown />
        </span>
      </button>

      {isOpen && (
        <ul className="mt-1 space-y-1">
          {visibleItems.map((item) => (
            <li key={item.href}>
              <Link
                to={item.href}
                className={`flex items-center px-3 py-2 text-sm rounded-md transition-colors ${
                  isActive(item.href)
                    ? 'bg-sidebar-active text-white'
                    : 'text-gray-300 hover:bg-sidebar-hover hover:text-white'
                }`}
              >
                <span className="mr-3 text-gray-400">{item.icon}</span>
                {item.label}
              </Link>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

export function Sidebar() {
  const location = useLocation();
  const isActive = (href: string) => {
    if (href === '/') {
      return location.pathname === '/';
    }
    return location.pathname.startsWith(href);
  };

  return (
    <aside className="fixed inset-y-0 left-0 z-30 w-sidebar bg-sidebar flex flex-col">
      {/* Logo */}
      <div className="flex items-center h-16 px-4 border-b border-sidebar-border">
        <Link to="/" className="flex items-center space-x-2">
          <div className="w-8 h-8 bg-primary rounded-lg flex items-center justify-center">
            <span className="text-white font-bold text-lg">A</span>
          </div>
          <span className="text-white font-semibold text-lg">AI-AAS</span>
        </Link>
      </div>

      {/* Navigation */}
      <nav className="flex-1 px-2 py-4 overflow-y-auto">
        {navSections.map((section) => (
          <SidebarSection
            key={section.title}
            section={section}
            isActive={isActive}
          />
        ))}
      </nav>

      {/* Footer */}
      <div className="px-4 py-3 border-t border-sidebar-border">
        <div className="text-xs text-gray-500">
          <p>v1.0.0</p>
          <a
            href="/docs/api"
            className="text-gray-400 hover:text-gray-300 transition-colors"
          >
            API Reference
          </a>
        </div>
      </div>
    </aside>
  );
}
