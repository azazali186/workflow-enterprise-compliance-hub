import { LogOut, Menu, PanelLeftClose, PanelLeftOpen } from "lucide-react";
import { useLocation, useNavigate } from "react-router-dom";
import { useAuth } from "@/hooks/useAuth";
import { Avatar } from "@/components/ui/Avatar";
import { Dropdown } from "@/components/ui/Dropdown";

const TITLES: Record<string, string> = {
  "/": "Dashboard",
  "/compliances": "Compliances",
  "/alerts": "Alerts",
  "/audit-logs": "Audit log",
  "/users": "Users",
};

export interface HeaderProps {
  collapsed: boolean;
  onToggleCollapsed: () => void;
  onOpenMobile: () => void;
}

export function Header({ collapsed, onToggleCollapsed, onOpenMobile }: HeaderProps) {
  const { user, role, logout } = useAuth();
  const { pathname } = useLocation();
  const navigate = useNavigate();

  const title = TITLES[pathname] ?? "ComplianceHub";
  const displayName = user?.username ?? "Account";
  const displayRole = role?.name ?? "Member";

  return (
    <header className="sticky top-0 z-20 flex h-14 shrink-0 items-center justify-between gap-3 border-b border-slate-200/80 bg-white/85 px-4 backdrop-blur-md">
      <div className="flex min-w-0 items-center gap-2">
        <button
          type="button"
          onClick={onOpenMobile}
          className="rounded-lg p-2 text-slate-500 transition-colors hover:bg-slate-100 hover:text-slate-800 lg:hidden"
          aria-label="Open menu"
        >
          <Menu className="h-5 w-5" />
        </button>
        <button
          type="button"
          onClick={onToggleCollapsed}
          className="hidden rounded-lg p-2 text-slate-500 transition-colors hover:bg-slate-100 hover:text-slate-800 lg:inline-flex"
          aria-label={collapsed ? "Expand sidebar" : "Collapse sidebar"}
        >
          {collapsed ? <PanelLeftOpen className="h-5 w-5" /> : <PanelLeftClose className="h-5 w-5" />}
        </button>
        <h1 className="truncate text-sm font-semibold text-slate-900">{title}</h1>
      </div>

      <Dropdown
        align="right"
        trigger={({ open, toggle }) => (
          <button
            type="button"
            onClick={toggle}
            aria-expanded={open}
            aria-haspopup="menu"
            className="flex items-center gap-2.5 rounded-lg p-1.5 pr-2 transition-colors hover:bg-slate-100"
          >
            <Avatar name={displayName} />
            <span className="hidden text-left sm:block">
              <span className="block max-w-40 truncate text-sm font-medium text-slate-800">{displayName}</span>
              <span className="block text-xs text-slate-500">{displayRole}</span>
            </span>
          </button>
        )}
        items={[
          {
            key: "logout",
            label: "Sign out",
            icon: <LogOut className="h-4 w-4" aria-hidden="true" />,
            onSelect: () => {
              logout();
              navigate("/login", { replace: true });
            },
          },
        ]}
      />
    </header>
  );
}
