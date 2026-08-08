import { useEffect, useState, type FormEvent } from "react";
import { Button } from "@/components/ui/Button";
import { Field } from "@/components/ui/Field";
import { Input } from "@/components/ui/Input";
import { Modal } from "@/components/ui/Modal";
import { SearchableSelect } from "@/components/ui/SearchableSelect";
import { Select } from "@/components/ui/Select";
import { Textarea } from "@/components/ui/Textarea";
import { statusMeta, severityMeta } from "@/lib/constants";
import { ApiError } from "@/types/api";
import { SEVERITIES, VIOLATION_STATUSES, type Violation } from "@/types/entities";

export interface ViolationFormValues {
  title: string;
  description: string;
  severity: string;
  status: string;
  compliance_id: string;
  regulation_id: string;
}

export interface ViolationFormModalProps {
  open: boolean;
  onClose: () => void;
  violation: Violation | null;
  onSubmit: (values: ViolationFormValues) => Promise<void>;
  submitting: boolean;
}

export function ViolationFormModal({ open, onClose, violation, onSubmit, submitting }: ViolationFormModalProps) {
  const isEdit = Boolean(violation);
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [severity, setSeverity] = useState("medium");
  const [status, setStatus] = useState("open");
  const [complianceId, setComplianceId] = useState("");
  const [regulationId, setRegulationId] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [submitError, setSubmitError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    setTitle(violation?.title ?? "");
    setDescription(violation?.description ?? "");
    setSeverity(violation?.severity ?? "medium");
    setStatus(violation?.status ?? "open");
    setComplianceId(violation?.compliance_id ?? "");
    setRegulationId(violation?.regulation_id ?? "");
    setErrors({});
    setSubmitError(null);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, violation?.id]);

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
        severity,
        status,
        compliance_id: complianceId.trim(),
        regulation_id: regulationId.trim(),
      });
    } catch (err) {
      setSubmitError(ApiError.is(err) ? err.userMessage : "Something went wrong. Please try again.");
    }
  };

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={isEdit ? "Edit violation" : "New violation"}
      description="A detected compliance violation, optionally tied to a compliance record and regulation."
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={submitting}>
            Cancel
          </Button>
          <Button type="submit" form="violation-form" loading={submitting}>
            {isEdit ? "Save changes" : "Create violation"}
          </Button>
        </>
      }
    >
      <form id="violation-form" onSubmit={submit} className="space-y-4" noValidate>
        {submitError && (
          <div className="rounded-lg border border-danger-200 bg-danger-50 px-3.5 py-2.5 text-sm text-danger-700" role="alert">
            {submitError}
          </div>
        )}

        <Field label="Title" htmlFor="violation-title" required error={errors.title}>
          <Input id="violation-title" value={title} onChange={(e) => setTitle(e.target.value)} autoFocus />
        </Field>

        <Field label="Description" htmlFor="violation-description">
          <Textarea id="violation-description" rows={3} value={description} onChange={(e) => setDescription(e.target.value)} />
        </Field>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Severity" htmlFor="violation-severity">
            <Select
              id="violation-severity"
              value={severity}
              onChange={(e) => setSeverity(e.target.value)}
              options={SEVERITIES.map((s) => ({ value: s, label: severityMeta[s]?.label ?? s }))}
            />
          </Field>
          <Field label="Status" htmlFor="violation-status">
            <Select
              id="violation-status"
              value={status}
              onChange={(e) => setStatus(e.target.value)}
              options={VIOLATION_STATUSES.map((s) => ({ value: s, label: statusMeta[s]?.label ?? s }))}
            />
          </Field>
        </div>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Compliance" htmlFor="violation-compliance" hint="Optional compliance record.">
            <SearchableSelect id="violation-compliance" entity="compliances" value={complianceId} onChange={setComplianceId} placeholder="Search compliance…" />
          </Field>
          <Field label="Regulation" htmlFor="violation-regulation" hint="Optional governing regulation.">
            <SearchableSelect id="violation-regulation" entity="regulations" value={regulationId} onChange={setRegulationId} placeholder="Search regulation…" />
          </Field>
        </div>
      </form>
    </Modal>
  );
}
