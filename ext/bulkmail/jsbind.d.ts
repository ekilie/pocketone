// ---- ext/bulkmail: $bulkmail ----

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
  pause(id: string): core.Record;
  resume(id: string): core.Record;
  cancel(id: string): core.Record;
  /** Sends one test message (to the given address, or the sender). */
  test(id: string, to?: string): void;
  /** Renders the campaign for one recipient. */
  preview(id: string, email?: string): bulkmail.Preview;
  /** Counts an audience: total, how many are suppressed, and a few sample addresses. */
  count(audience: bulkmail.Audience): { count: number; suppressed: number; sample: string[] };
  suppress(email: string, reason?: string): core.Record;
  unsuppress(email: string): void;
  isSuppressed(email: string): boolean;
  /** Suppresses the address and fires the unsubscribe hooks. */
  unsubscribe(email: string, campaignId?: string): void;
  unsubscribeURL(email: string, campaignId?: string): string;
  /** Sending limits: messages per second, per day, and sent today. */
  limits(): { rate: number; daily: number; sentToday: number };
};
