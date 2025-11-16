import { promises as fs } from 'node:fs';
export class PolicyEngine {
    allowed;
    constructor(allowed) {
        this.allowed = allowed;
    }
    static async fromFile(path) {
        const raw = await fs.readFile(path, 'utf-8');
        return PolicyEngine.fromString(raw);
    }
    static fromString(source) {
        const doc = JSON.parse(source);
        const allowed = new Map();
        Object.entries(doc.rules ?? {}).forEach(([action, roles]) => {
            const normalized = action.toUpperCase();
            const set = new Set();
            roles.forEach((role) => set.add(role.trim().toLowerCase()));
            allowed.set(normalized, set);
        });
        return new PolicyEngine(allowed);
    }
    allowedRoles(action) {
        return this.allowed.get(action.toUpperCase());
    }
    isAllowed(action, roles) {
        const set = this.allowedRoles(action);
        if (!set || set.size === 0) {
            return false;
        }
        return roles.some((role) => set.has(role.trim().toLowerCase()));
    }
}
//# sourceMappingURL=policy.js.map