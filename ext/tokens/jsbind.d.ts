// ---- ext/tokens: $tokens ----

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
 * $tokens.registerScope({ name: "notes:write", description: "Change notes", paths: ["/api/collections/notes/records"] })
 * routerAdd("POST", "/api/notes/archive", (e) => { ... }, $apis.requireAuth(), $tokens.requireScope("notes:write"))
 * ```
 *
 * @group PocketOne
 */
declare var $tokens: {
  /** Registers a scope (top level of a pb_hooks file); the same name again merges. */
  registerScope(def: tokens.ScopeDef): void;
  /** Route middleware: a token, if the request uses one, must hold the scope. */
  requireScope(name: string): any;
  /** Whether the request may use the scope (always true without a token). */
  hasScope(e: core.RequestEvent, name: string): boolean;
  /** The collection-rule field for the scope. */
  ruleField(name: string): string;
  /** Every registered scope. */
  scopes(): tokens.Scope[];
};
