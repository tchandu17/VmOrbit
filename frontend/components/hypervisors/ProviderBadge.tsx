import { cn } from "@/lib/utils";
import type { ProviderType } from "@/types";

interface ProviderBadgeProps {
  provider: ProviderType;
  className?: string;
}

const PROVIDER_LABELS: Record<ProviderType, string> = {
  vmware:  "VMware vCenter",
  esxi:    "VMware ESXi",
  proxmox: "Proxmox VE",
  nutanix: "Nutanix AHV",
  kvm:     "KVM",
  hyperv:  "Hyper-V",
};

const PROVIDER_COLORS: Record<ProviderType, string> = {
  vmware:  "bg-blue-500/15 text-blue-400 border-blue-500/30",
  esxi:    "bg-blue-500/15 text-blue-300 border-blue-400/30",
  proxmox: "bg-orange-500/15 text-orange-400 border-orange-500/30",
  nutanix: "bg-teal-500/15 text-teal-400 border-teal-500/30",
  kvm:     "bg-green-500/15 text-green-400 border-green-500/30",
  hyperv:  "bg-purple-500/15 text-purple-400 border-purple-500/30",
};

export function ProviderBadge({ provider, className }: ProviderBadgeProps) {
  const label = PROVIDER_LABELS[provider] ?? provider;
  const color = PROVIDER_COLORS[provider] ?? "bg-gray-500/15 text-gray-400 border-gray-500/30";

  return (
    <span
      className={cn(
        "inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-semibold uppercase tracking-wide border",
        color,
        className
      )}
    >
      {label}
    </span>
  );
}
