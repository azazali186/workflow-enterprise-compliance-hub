import { http } from "./api/client";
import { API } from "./api/paths";
import type { BusEvent } from "@/types/entities";

export interface NotificationSendBody {
  type: string;
  title: string;
  message?: string;
  severity?: string;
  entity_type?: string;
  entity_id?: string;
}

export interface NotificationSendResult {
  sent: boolean;
  event_type: string;
  subject: string;
}

export const notificationsApi = {
  /** Queues a notification through the outbox → bus → WebSocket clients. */
  send: (body: NotificationSendBody): Promise<NotificationSendResult> => http.post<NotificationSendResult>(API.notificationsSend, body),

  /** Recent events from the in-process bus ring. */
  recent: (limit?: number, signal?: AbortSignal): Promise<BusEvent[]> =>
    http.post<BusEvent[]>(API.notificationsEvents, { limit: limit ?? 50 }, signal),
};
