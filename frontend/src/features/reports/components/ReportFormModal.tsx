import { useEffect, useState, type FormEvent } from "react";
import { Button } from "@/components/ui/Button";
import { Field } from "@/components/ui/Field";
import { Input } from "@/components/ui/Input";
import { Modal } from "@/components/ui/Modal";
import { SearchableSelect } from "@/components/ui/SearchableSelect";
import { Select } from "@/components/ui/Select";
import { Textarea } from "@/components/ui/Textarea";
import { reportTypeLabel, statusMeta } from "@/lib/constants";
import { ApiError } from "@/types/api";
import { REPORT_STATUSES, REPORT_TYPES, type Report } from "@/types/entities";

export interface ReportFormValues {
  title: string;
  type: string;
  status: string;
  description: string;
  compliance_id: string;
}

export interface ReportFormModalProps {
  open: boolean;
  onClose: () => void;
  /** null + mode "generate" = generate endpoint; otherwise PATCH on this report. */
  report: Report | null;
  mode: "generate" | "edit";
  onSubmit: (values: ReportFormValues) => Promise<void>;
  submitting: boolean;
}

export function ReportFormModal({ open, onClose, report, mode, onSubmit, submitting }: ReportFormModalProps) {
  const isGenerate = mode === "generate";
  const [title, setTitle] = useState("");
  const [type, setType] = useState("summary");
  const [status, setStatus] = useState("generated");
  const [description, setDescription] = useState("");
  const [complianceId, setComplianceId] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [submitError, setSubmitError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    setTitle(report?.title ?? "");
    setType(report?.type ?? "summary");
    setStatus(report?.status ?? "generated");
    setDescription(report?.description ?? "");
    setComplianceId(report?.compliance_id ?? "");
    setErrors({});
    setSubmitError(null);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, report?.id, mode]);

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
        type,
        status,
        description,
        compliance_id: complianceId.trim(),
      });
    } catch (err) {
      setSubmitError(ApiError.is(err) ? err.userMessage : "Something went wrong. Please try again.");
    }
  };

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={isGenerate ? "Generate report" : "Edit report"}
      description={
        isGenerate
          ? "Assembles a summary of audits, violations, alerts and checklists for a compliance entity."
          : "Update the report's metadata."
      }
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={submitting}>
            Cancel
          </Button>
          <Button type="submit" form="report-form" loading={submitting}>
            {isGenerate ? "Generate report" : "Save changes"}
          </Button>
        </>
      }
    >
      <form id="report-form" onSubmit={submit} className="space-y-4" noValidate>
        {submitError && (
          <div className="rounded-lg border border-danger-200 bg-danger-50 px-3.5 py-2.5 text-sm text-danger-700" role="alert">
            {submitError}
          </div>
        )}

        <Field label="Title" htmlFor="report-title" required error={errors.title}>
          <Input id="report-title" value={title} onChange={(e) => setTitle(e.target.value)} autoFocus />
        </Field>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Type" htmlFor="report-type">
            <Select
              id="report-type"
              value={type}
              onChange={(e) => setType(e.target.value)}
              options={REPORT_TYPES.map((t) => ({ value: t, label: reportTypeLabel(t) }))}
            />
          </Field>
          {!isGenerate && (
            <Field label="Status" htmlFor="report-status">
              <Select
                id="report-status"
                value={status}
                onChange={(e) => setStatus(e.target.value)}
                options={REPORT_STATUSES.map((s) => ({ value: s, label: statusMeta[s]?.label ?? s }))}
              />
            </Field>
          )}
        </div>

        <Field label="Description" htmlFor="report-description">
          <Textarea id="report-description" rows={2} value={description} onChange={(e) => setDescription(e.target.value)} />
        </Field>

        <Field label="Compliance" htmlFor="report-compliance" hint="Optional — leave empty for a global report.">
          <SearchableSelect id="report-compliance" entity="compliances" value={complianceId} onChange={setComplianceId} placeholder="Search compliance…" />
        </Field>
      </form>
    </Modal>
  );
}
