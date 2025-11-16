export interface RequestWithHeaders {
    headers: Record<string, unknown>;
    id?: string;
}
export interface ReplyWithHeader {
    header: (name: string, value: string) => void;
}
export declare function createRequestContextHook(): (request: RequestWithHeaders, reply: ReplyWithHeader) => Promise<void>;
