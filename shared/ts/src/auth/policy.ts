import { promises as fs } from 'node:fs';

export interface PolicyDocument {
  rules: Record<string, string[]>;
}

export class PolicyEngine {
  private readonly allowed: Map<string, Set<string>>;

  private constructor(allowed: Map<string, Set<string>>) {
    this.allowed = allowed;
  }

  static async fromFile(path: string): Promise<PolicyEngine> {
    const raw = await fs.readFile(path, 'utf-8');
    return PolicyEngine.fromString(raw);
  }

  static fromString(source: string): PolicyEngine {
    const doc = JSON.parse(source) as PolicyDocument;
    const allowed = new Map<string, Set<string>>();
    Object.entries(doc.rules ?? {}).forEach(([action, roles]) => {
      const normalized = action.toUpperCase();
      const set = new Set<string>();
      roles.forEach((role) => set.add(role.trim().toLowerCase()));
      allowed.set(normalized, set);
    });
    return new PolicyEngine(allowed);
  }

  allowedRoles(action: string): Set<string> | undefined {
    return this.allowed.get(action.toUpperCase());
  }

  isAllowed(action: string, roles: string[]): boolean {
    const set = this.allowedRoles(action);
    if (!set || set.size === 0) {
      return false;
    }
    return roles.some((role) => set.has(role.trim().toLowerCase()));
  }
}

