import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Bell, ChevronDown, ChevronUp, RefreshCw, Send } from "lucide-react";
import { useState, type FormEvent } from "react";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Field } from "@/components/ui/Field";
import { Input } from "@/components/ui/Input";
import { PageHeader } from "@/components/ui/PageHeader";
import { Select } from "@/components/ui/Select";
import { Textarea } from "@/components/ui/Textarea";
import { useAuth } from "@/hooks/useAuth";
import { severityMeta } from "@/lib/constants";
import { formatDateTime } from "@/lib/format";
import { PERM } from "@/services/api/paths";
import { notificationsApi } from "@/services/notifications.service";
import { toast } from "@/store/toast.slice";
import { ApiError } from "@/types/api";
import { SEVERITIES } from "@/types/entities";
import { cn } from "@/lib/cn";

const EVENT_TYPES = ["compliance_alert", "deadline_approaching", "violation_detected", "audit_scheduled", "notification_sent", "info"];

export function NotificationsPage() {
  const { can } = useAuth();
  const qc = useQueryClient();
  const canSend = can(PERM.notificationsSend);
  const canView = can(PERM.notificationsEvents);

  const [type, setType] = useState("info");
  const [title, setTitle] = useState("");
  const [message, setMessage] = useState("");
  const [severity, setSeverity] = useState("low");
  const [entityType, setEntityType] = useState("");
  const [entityId, setEntityId] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [expanded, setExpanded] = useState<string | null>(null);

  const events = useQuery({
    queryKey: ["notifications-events"],
    queryFn: ({ signal }) => notificationsApi.recent(50, signal),
    enabled: canView,
    refetchInterval: 10_000,
  });

  const sendMutation = useMutation({
    mutationFn: notificationsApi.send,
    onSuccess: (res) => {
      toast.success("Notification sent", `${res.event_type} · ${res.subject}`);
      void qc.invalidateQueries({ queryKey: ["notifications-events"] });
    },
    onError: (e) => toast.error("Could not send notification", ApiError.is(e) ? e.userMessage : undefined),
  });

  const submit = async (ev: FormEvent) => {
    ev.preventDefault();
    const e: Record<string, string> = {};
    if (!type.trim()) e.type = "Type is required.";
    if (!title.trim()) e.title = "Title is required.";
    setErrors(e);
    if (Object.keys(e).length > 0) return;
    try {
      await sendMutation.mutateAsync({
        type: type.trim(),
        title: title.trim(),
        message,
        severity,
        entity_type: entityType.trim() || undefined,
        entity_id: entityId.trim() || undefined,
      });
      setTitle("");
      setMessage("");
    } catch {
      // onError already toasts; swallow the rejected promise from the form handler.
    }
  };

  return (
    <div className="space-y-5">
      <PageHeader
        title="Notifications"
        description="Send notifications through the outbox → event bus → WebSocket pipeline."
        actions={
          canView ? (
            <Button variant="secondary" icon={<RefreshCw className="h-4 w-4" />} onClick={() => void events.refetch()} loading={events.isFetching}>
              Refresh events
            </Button>
          ) : undefined
        }
      />

      <div className="grid grid-cols-1 gap-5 xl:grid-cols-5">
        {canSend && (
        <Card className="xl:col-span-2">
          <div className="border-b border-slate-100 px-5 py-4">
            <h2 className="flex items-center gap-2 text-sm font-semibold text-slate-900">
              <Send className="h-4 w-4 text-primary-600" aria-hidden="true" />
              Send notification
            </h2>
          </div>
          <form onSubmit={submit} className="space-y-4 p-5" noValidate>
            {errors.title || errors.type ? (
              <div className="rounded-lg border border-danger-200 bg-danger-50 px-3.5 py-2.5 text-sm text-danger-700" role="alert">
                {errors.type || errors.title}
              </div>
            ) : null}
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <Field label="Type" htmlFor="notif-type" required error={errors.type}>
                <Select
                  id="notif-type"
                  value={type}
                  onChange={(e) => setType(e.target.value)}
                  options={EVENT_TYPES.map((t) => ({ value: t, label: t.replace(/_/g, " ") }))}
                />
              </Field>
              <Field label="Severity" htmlFor="notif-severity">
                <Select
                  id="notif-severity"
                  value={severity}
                  onChange={(e) => setSeverity(e.target.value)}
                  options={SEVERITIES.map((s) => ({ value: s, label: severityMeta[s]?.label ?? s }))}
                />
              </Field>
            </div>
            <Field label="Title" htmlFor="notif-title" required error={errors.title}>
              <Input id="notif-title" value={title} onChange={(e) => setTitle(e.target.value)} />
            </Field>
            <Field label="Message" htmlFor="notif-message">
              <Textarea id="notif-message" rows={3} value={message} onChange={(e) => setMessage(e.target.value)} />
            </Field>
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <Field label="Entity type" htmlFor="notif-entity-type" hint="Optional context link.">
                <Input id="notif-entity-type" value={entityType} onChange={(e) => setEntityType(e.target.value)} placeholder="e.g. compliance" />
              </Field>
              <Field label="Entity id" htmlFor="notif-entity-id">
                <Input id="notif-entity-id" value={entityId} onChange={(e) => setEntityId(e.target.value)} placeholder="00000000-0000-0000-0000-000000000000" />
              </Field>
            </div>
            <Button type="submit" loading={sendMutation.isPending} className="w-full">
              Send notification
            </Button>
          </form>
        </Card>
        )}

        <Card className="xl:col-span-3">
          <div className="flex items-center justify-between border-b border-slate-100 px-5 py-4">
            <h2 className="flex items-center gap-2 text-sm font-semibold text-slate-900">
              <Bell className="h-4 w-4 text-primary-600" aria-hidden="true" />
              Recent events
              <span className="rounded-full bg-slate-100 px-2 py-0.5 text-xs font-medium text-slate-500">{events.data?.length ?? 0}</span>
            </h2>
          </div>

          <div className="divide-y divide-slate-100">
            {events.isError ? (
              <div className="px-5 py-8 text-center text-sm text-slate-500">Could not load recent events.</div>
            ) : events.isLoading ? (
              <div className="space-y-2.5 p-5">
                {[0, 1, 2, 3].map((i) => (
                  <div key={i} className="h-14 animate-pulse rounded-lg bg-slate-100" />
                ))}
              </div>
            ) : events.data && events.data.length > 0 ? (
              events.data.map((ev) => {
                const open = expanded === ev.id;
                const hasPayload = ev.payload !== undefined && ev.payload !== null;
                return (
                  <div key={ev.id} className="px-5 py-3.5">
                    <div className="flex flex-wrap items-center justify-between gap-2">
                      <div className="flex min-w-0 items-center gap-2.5">
                        <Badge tone="neutral">{ev.subject}</Badge>
                        <span className="truncate font-medium text-slate-800">{ev.type}</span>
                      </div>
                      <div className="flex shrink-0 items-center gap-2">
                        <span className="text-xs text-slate-400">{formatDateTime(ev.timestamp)}</span>
                        {hasPayload && (
                          <button
                            type="button"
                            onClick={() => setExpanded(open ? null : ev.id)}
                            className="rounded-md p-1 text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-700"
                            aria-label={open ? "Collapse payload" : "Expand payload"}
                          >
                            {open ? <ChevronUp className="h-4 w-4" /> : <ChevronDown className="h-4 w-4" />}
                          </button>
                        )}
                      </div>
                    </div>
                    {open && hasPayload && (
                      <pre className={cn("mt-3 max-h-52 overflow-auto rounded-lg bg-slate-50 p-3 font-mono text-xs leading-relaxed text-slate-700")}>
                        {JSON.stringify(ev.payload, null, 2)}
                      </pre>
                    )}
                  </div>
                );
              })
            ) : (
              <div className="px-5 py-10 text-center">
                <Bell className="mx-auto h-8 w-8 text-slate-300" aria-hidden="true" />
                <p className="mt-3 text-sm font-medium text-slate-700">No events yet</p>
                <p className="mt-1 text-xs text-slate-500">Events published on this process will appear here.</p>
              </div>
            )}
          </div>
        </Card>
      </div>
    </div>
  );
}
