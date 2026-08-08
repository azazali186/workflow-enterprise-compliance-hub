import { AnimatePresence, motion } from "framer-motion";
import {
  AlertTriangle,
  BarChart3,
  Bell,
  CalendarClock,
  ClipboardCheck,
  FileText,
  LayoutDashboard,
  ListChecks,
  Scale,
  ScrollText,
  Send,
  ShieldCheck,
  Users,
  Workflow,
  Wrench,
  X,
  type LucideIcon,
} from "lucide-react";
import { NavLink, useLocation } from "react-router-dom";
import { useAuth } from "@/hooks/useAuth";
import { cn } from "@/lib/cn";
import { PERM } from "@/services/api/paths";

interface NavItem {
  to: string;
  label: string;
  icon: LucideIcon;
  /** Route-level permission required to see the item (admin bypasses). */
  perm?: string;
}

interface NavSection {
  label: string;
  items: NavItem[];
}

const NAV_SECTIONS: NavSection[] = [
  {
    label: "Overview",
    items: [{ to: "/", label: "Dashboard", icon: LayoutDashboard }],
  },
  {
    label: "Compliance",
    items: [
      { to: "/compliances", label: "Compliances", icon: ShieldCheck, perm: PERM.compliancesSearch },
      { to: "/regulations", label: "Regulations", icon: Scale, perm: PERM.regulationsSearch },
      { to: "/checklists", label: "Checklists", icon: ListChecks, perm: PERM.checklistsSearch },
      { to: "/audits", label: "Audits", icon: ClipboardCheck, perm: PERM.auditsSearch },
      { to: "/violations", label: "Violations", icon: AlertTriangle, perm: PERM.violationsSearch },
    ],
  },
  {
    label: "Operations",
    items: [
      { to: "/alerts", label: "Alerts", icon: Bell, perm: PERM.alertsSearch },
      { to: "/deadlines", label: "Deadlines", icon: CalendarClock, perm: PERM.deadlinesSearch },
      { to: "/corrective-actions", label: "Corrective actions", icon: Wrench, perm: PERM.correctiveActionsSearch },
      { to: "/reports", label: "Reports", icon: FileText, perm: PERM.reportsSearch },
      { to: "/notifications", label: "Notifications", icon: Send, perm: PERM.notificationsSend },
    ],
  },
  {
    label: "Intelligence",
    items: [
      { to: "/analytics", label: "Analytics", icon: BarChart3, perm: PERM.analyticsSummary },
      { to: "/sagas", label: "Sagas", icon: Workflow, perm: PERM.sagasSearch },
    ],
  },
  {
    label: "Administration",
    items: [
      { to: "/audit-logs", label: "Audit log", icon: ScrollText, perm: PERM.auditLogsSearch },
      { to: "/users", label: "Users", icon: Users, perm: PERM.usersSearch },
    ],
  },
];

export interface SidebarProps {
  collapsed: boolean;
  /** Mobile drawer open state (ignored on desktop). */
  mobileOpen: boolean;
  onCloseMobile: () => void;
}

export function Sidebar({ collapsed, mobileOpen, onCloseMobile }: SidebarProps) {
  const { can } = useAuth();
  const { pathname } = useLocation();

  const sections = NAV_SECTIONS.map((s) => ({
    ...s,
    items: s.items.filter((i) => !i.perm || can(i.perm)),
  })).filter((s) => s.items.length > 0);

  const body = (
    <div className="flex h-full flex-col bg-ink-900 text-slate-300">
      <div className={cn("flex h-14 items-center gap-3 border-b border-white/5 px-4", collapsed && "justify-center px-0")}>
        <span className="brand-mark flex h-8 w-8 shrink-0 items-center justify-center rounded-lg" aria-hidden="true">
          <ShieldCheck className="h-4.5 w-4.5 text-white" />
        </span>
        {!collapsed && (
          <div className="min-w-0">
            <p className="truncate text-sm font-bold tracking-tight text-white">ComplianceHub</p>
            <p className="truncate text-[11px] text-slate-400">Governance console</p>
          </div>
        )}
      </div>

      <nav className="flex-1 overflow-y-auto px-3 py-4" aria-label="Primary">
        <div className="space-y-5">
          {sections.map((section) => (
            <div key={section.label}>
              {!collapsed && (
                <p className="mb-1.5 px-2 text-[11px] font-semibold uppercase tracking-wider text-slate-500">
                  {section.label}
                </p>
              )}
              <ul className="space-y-0.5">
                {section.items.map((item) => (
                  <NavItemRow key={item.to} item={item} collapsed={collapsed} active={pathname === item.to} />
                ))}
              </ul>
            </div>
          ))}
        </div>
      </nav>

      <div className="border-t border-white/5 p-3">
        <button
          type="button"
          onClick={onCloseMobile}
          className={cn(
            "flex w-full items-center gap-2.5 rounded-lg px-2.5 py-2 text-sm text-slate-400 transition-colors hover:bg-white/5 hover:text-white lg:hidden",
            collapsed && "justify-center px-0",
          )}
        >
          <X className="h-4.5 w-4.5 shrink-0" aria-hidden="true" />
          {!collapsed && "Close menu"}
        </button>
      </div>
    </div>
  );

  return (
    <>
      {/* Desktop */}
      <aside
        className={cn(
          "hidden h-dvh shrink-0 transition-[width] duration-200 lg:block",
          collapsed ? "w-[68px]" : "w-60",
        )}
      >
        {body}
      </aside>

      {/* Mobile drawer */}
      <AnimatePresence>
        {mobileOpen && (
          <div className="fixed inset-0 z-50 lg:hidden">
            <motion.div
              className="absolute inset-0 bg-slate-950/50 backdrop-blur-[2px]"
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              transition={{ duration: 0.15 }}
              onClick={onCloseMobile}
              aria-hidden="true"
            />
            <motion.div
              className="absolute inset-y-0 left-0 w-64"
              initial={{ x: "-100%" }}
              animate={{ x: 0 }}
              exit={{ x: "-100%" }}
              transition={{ type: "spring", stiffness: 380, damping: 34 }}
            >
              {body}
            </motion.div>
          </div>
        )}
      </AnimatePresence>
    </>
  );
}

function NavItemRow({ item, collapsed, active }: { item: NavItem; collapsed: boolean; active: boolean }) {
  const Icon = item.icon;
  return (
    <li>
      <NavLink
        to={item.to}
        aria-current={active ? "page" : undefined}
        title={collapsed ? item.label : undefined}
        className={cn(
          "relative flex items-center gap-2.5 rounded-lg px-2.5 py-2 text-sm font-medium transition-colors",
          active ? "text-white" : "text-slate-400 hover:bg-white/5 hover:text-slate-100",
          collapsed && "justify-center px-0",
        )}
      >
        {active && (
          <motion.span
            layoutId="sidebar-active"
            className="absolute inset-0 rounded-lg bg-white/10 ring-1 ring-inset ring-white/10"
            transition={{ type: "spring", stiffness: 420, damping: 32 }}
            aria-hidden="true"
          />
        )}
        <Icon className="relative z-10 h-4.5 w-4.5 shrink-0" aria-hidden="true" />
        {!collapsed && <span className="relative z-10 truncate">{item.label}</span>}
      </NavLink>
    </li>
  );
}
