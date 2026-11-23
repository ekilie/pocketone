// ---- ext/api-tokens: $apiTokens ----

/** API Tokens' types: the scopes API tokens and OAuth clients can be granted (a scope's definition, and a scope as registered). */
declare namespace tokens {
  /** A scope API tokens and OAuth clients can be granted. */
  interface ScopeDef {
    /** e.g. "notes:read". */
    name: string;
    description?: string;
    /** Path prefixes the scope covers. */
    paths?: string[];
    /** HTTP methods it covers (all when empty). */
    methods?: string[];
    /** Collections whose record APIs it covers. */
    collections?: string[];
    /**
     * Lets a token hold the scope for some values only, written "name=value" (e.g. "notes=abc123"); the bare name
     * still means every value. Routes check the value with hasScope(e, name, value).
     */
    values?: { description: string };
  }
  interface Scope extends ScopeDef {
    /** The request header a token's scope shows up in. */
    header: string;
    /** The field to use in collection rules, e.g. "@request.headers.x_token_scope_notes_read". */
    ruleField: string;
  }
}

/**
 * API token scopes.
 *
 * ```js
 * $apiTokens.registerScope({ name: "notes:write", description: "Change notes", paths: ["/api/collections/notes/records"] })
 * routerAdd("POST", "/api/notes/archive", (e) => { ... }, $apis.requireAuth(), $apiTokens.requireScope("notes:write"))
 * ```
 *
 * @group PocketOne
 */
declare var $apiTokens: {
  /** Registers a scope (top level of a pb_hooks file); the same name again merges. */
  registerScope(def: tokens.ScopeDef): void;
  /** Route middleware: a token, if the request uses one, must hold the scope. */
  requireScope(name: string): any;
  /**
   * Whether the request may use the scope (always true without a token). With a value: whether it may use the scope
   * on that value (it holds the bare scope, or "name=value").
   */
  hasScope(e: core.RequestEvent, name: string, value?: string): boolean;
  /** The values the request holds the scope for: all is true for every value. */
  scopeValues(e: core.RequestEvent, name: string): { all: boolean; values: string[] };
  /** The collection-rule field for the scope. */
  ruleField(name: string): string;
  /** Every registered scope. */
  scopes(): tokens.Scope[];
};
