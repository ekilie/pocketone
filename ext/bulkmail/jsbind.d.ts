// ---- ext/bulkmail: $bulkmail ----

/** Bulk Mail's types: who a campaign goes to (an audience) and a rendered preview. */
declare namespace bulkmail {
  /** Who a campaign goes to: records of a collection, or a read-only SQL query. */
  type Audience = { collection: string; filter?: string; email_field?: string } | { sql: string };
  interface Preview {
    to: string;
    from: string;
    subject: string;
    html: string;
    text: string;
    headers: { [key: string]: string };
  }
}

/**
 * Bulk mail campaigns. To react to its activity, use record hooks on its collections, e.g.
 * `onRecordAfterCreateSuccess(e => ..., "_bulkmail_suppressions")`.
 *
 * @group PocketOne
 */
declare var $bulkmail: {
  /** Starts a campaign (at most limit sends; 0 = all); returns the campaign record. */
  start(id: string, limit?: number): core.Record;
  /** Stops sending (or a scheduled start) until resume; returns the campaign record. */
  pause(id: string): core.Record;
  /** Continues a paused campaign; returns the campaign record. */
  resume(id: string): core.Record;
  /** Stops a campaign for good: recipients still queued are marked skipped. Returns the campaign record. */
  cancel(id: string): core.Record;
  /** Sends one test message (to the given address, or the sender). */
  test(id: string, to?: string): void;
  /** Renders the campaign for one recipient. */
  preview(id: string, email?: string): bulkmail.Preview;
  /** Counts an audience: total, how many are suppressed, and a few sample addresses. */
  count(audience: bulkmail.Audience): { count: number; suppressed: number; sample: string[] };
  /** Adds the address to the suppression list (or updates its reason), so campaigns skip it. Fires no hooks: unsubscribe does. */
  suppress(email: string, reason?: string): core.Record;
  /** Takes the address off the suppression list, so campaigns reach it again. */
  unsuppress(email: string): void;
  /** Whether campaigns skip the address (it's on the suppression list). */
  isSuppressed(email: string): boolean;
  /** Suppresses the address and fires the unsubscribe hooks. */
  unsubscribe(email: string, campaignId?: string): void;
  /** The address's unsubscribe link, as every campaign message and its List-Unsubscribe header carry it. */
  unsubscribeURL(email: string, campaignId?: string): string;
  /** Sending limits: messages per second, per day, and sent today. */
  limits(): { rate: number; daily: number; sentToday: number };
};
