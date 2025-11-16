export interface PolicyDocument {
    rules: Record<string, string[]>;
}
export declare class PolicyEngine {
    private readonly allowed;
    private constructor();
    static fromFile(path: string): Promise<PolicyEngine>;
    static fromString(source: string): PolicyEngine;
    allowedRoles(action: string): Set<string> | undefined;
    isAllowed(action: string, roles: string[]): boolean;
}
