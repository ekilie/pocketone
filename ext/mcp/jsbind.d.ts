// ---- ext/mcp: $mcp ----

/** MCP's types: the tools /mcp offers, each one REST endpoint, and the requests they make. */
declare namespace mcp {
  /** The REST request a tool makes; a request transform may change it. */
  interface ToolRequest {
    method: string;
    path: string;
    query?: { [key: string]: string };
    body?: any;
    headers?: { [key: string]: string };
  }
  /** One MCP tool, mapped to one REST endpoint. */
  interface Endpoint {
    /** Tool name, e.g. "blog_publish_post". */
    name: string;
    title?: string;
    description: string;
    /** HTTP method of the endpoint. */
    method: string;
    /** Endpoint path; {name} parts come from the tool's arguments, the rest go to the JSON body. */
    path: string;
    /** JSON Schema of the tool's arguments. */
    inputSchema?: { [key: string]: any };
    /** JSON Schema of the tool's result. */
    outputSchema?: { [key: string]: any };
    annotations?: {
      title?: string;
      readOnlyHint?: boolean;
      destructiveHint?: boolean;
      idempotentHint?: boolean;
      openWorldHint?: boolean;
    };
    /** Shapes the request (it can't change who the call runs as). Return nothing to keep the default. */
    request?: (c: { args: { [key: string]: any }; auth: core.Record | null; request: ToolRequest }) => ToolRequest | void;
    /** Shapes the result the model sees; throw for an error the model can read. */
    response?: (r: {
      args: { [key: string]: any };
      auth: core.Record | null;
      status: number;
      headers: { [key: string]: string };
      body: string;
      json: any;
      truncated: boolean;
    }) => any;
  }
}

/**
 * The app's MCP server (`/mcp`): tools map to the app's own REST endpoints.
 *
 * ```js
 * $mcp.addEndpoint({ name: "blog_publish_post", description: "Publish a draft post.", method: "POST",
 *   path: "/api/blog/posts/{id}/publish", inputSchema: { type: "object", required: ["id"], properties: { id: { type: "string" } } } })
 * ```
 *
 * @group PocketOne
 */
declare var $mcp: {
  /** Declares a tool, at the top level of a pb_hooks file. Adding the same name again replaces it. */
  addEndpoint(def: mcp.Endpoint): void;
  /** Escapes a value for one {name} part of a tool's path. */
  pathValue(value: string): string;
  /** Adds text to the server's instructions for the model. */
  instructions(text: string): void;
  /** The names of every tool. */
  tools(): string[];
};
