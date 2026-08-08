import { useEffect, useState, type FormEvent } from "react";
import { Button } from "@/components/ui/Button";
import { Field } from "@/components/ui/Field";
import { Input } from "@/components/ui/Input";
import { Modal } from "@/components/ui/Modal";
import { Select } from "@/components/ui/Select";
import { Textarea } from "@/components/ui/Textarea";
import { statusMeta } from "@/lib/constants";
import { ApiError } from "@/types/api";
import { AUDIT_STATUSES, type Audit } from "@/types/entities";

export interface AuditFormValues {
  title: string;
  description: string;
  status: string;
  compliance_id: string;
  auditor_id: string;
  scheduled_at: string | null;
}

export interface AuditFormModalProps {
  open: boolean;
  onClose: () => void;
  audit: Audit | null;
  onSubmit: (values: AuditFormValues) => Promise<void>;
  submitting: boolean;
}

export function AuditFormModal({ open, onClose, audit, onSubmit, submitting }: AuditFormModalProps) {
  const isEdit = Boolean(audit);
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [status, setStatus] = useState("scheduled");
  const [complianceId, setComplianceId] = useState("");
  const [auditorId, setAuditorId] = useState("");
  const [scheduledAt, setScheduledAt] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [submitError, setSubmitError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    setTitle(audit?.title ?? "");
    setDescription(audit?.description ?? "");
    setStatus(audit?.status ?? "scheduled");
    setComplianceId(audit?.compliance_id ?? "");
    setAuditorId(audit?.auditor_id ?? "");
    setScheduledAt(audit?.scheduled_at ? audit.scheduled_at.slice(0, 16) : "");
    setErrors({});
    setSubmitError(null);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, audit?.id]);

  const submit = async (ev: FormEvent) => {
    ev.preventDefault();
    const e: Record<string, string> = {};
    if (!title.trim()) e.title = "Title is required.";
    setErrors(e);
    if (Object.keys(e).length > 0) return;
    setSubmitError(null);
    try {
      await onSubmit({
        title: title.trim(),
        description,
        status,
        compliance_id: complianceId.trim(),
        auditor_id: auditorId.trim(),
        scheduled_at: scheduledAt ? `${scheduledAt}:00.000Z` : null,
      });
    } catch (err) {
      setSubmitError(ApiError.is(err) ? err.userMessage : "Something went wrong. Please try again.");
    }
  };

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={isEdit ? "Edit audit" : "New audit"}
      description="A scheduled compliance audit that drives the AuditExecution saga."
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={submitting}>
            Cancel
          </Button>
          <Button type="submit" form="audit-form" loading={submitting}>
            {isEdit ? "Save changes" : "Create audit"}
          </Button>
        </>
      }
    >
      <form id="audit-form" onSubmit={submit} className="space-y-4" noValidate>
        {submitError && (
          <div className="rounded-lg border border-danger-200 bg-danger-50 px-3.5 py-2.5 text-sm text-danger-700" role="alert">
            {submitError}
          </div>
        )}

        <Field label="Title" htmlFor="audit-title" required error={errors.title}>
          <Input id="audit-title" value={title} onChange={(e) => setTitle(e.target.value)} autoFocus />
        </Field>

        <Field label="Description" htmlFor="audit-description">
          <Textarea id="audit-description" rows={2} value={description} onChange={(e) => setDescription(e.target.value)} />
        </Field>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Status" htmlFor="audit-status">
            <Select
              id="audit-status"
              value={status}
              onChange={(e) => setStatus(e.target.value)}
              options={AUDIT_STATUSES.map((s) => ({ value: s, label: statusMeta[s]?.label ?? s }))}
            />
          </Field>
          <Field label="Scheduled at" htmlFor="audit-scheduled">
            <Input id="audit-scheduled" type="datetime-local" value={scheduledAt} onChange={(e) => setScheduledAt(e.target.value)} />
          </Field>
        </div>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Compliance id" htmlFor="audit-compliance" hint="Optional UUID of the compliance record.">
            <Input id="audit-compliance" value={complianceId} onChange={(e) => setComplianceId(e.target.value)} placeholder="00000000-0000-0000-0000-000000000000" />
          </Field>
          <Field label="Auditor" htmlFor="audit-auditor" hint="Optional auditor id.">
            <Input id="audit-auditor" value={auditorId} onChange={(e) => setAuditorId(e.target.value)} />
          </Field>
        </div>
      </form>
    </Modal>
  );
}
