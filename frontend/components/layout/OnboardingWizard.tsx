"use client";
import { useState } from "react";
import { useRouter } from "next/navigation";
import {
  Server,
  Cpu,
  RefreshCw,
  CheckCircle,
  ArrowRight,
  Rocket,
  Keyboard,
  BookOpen,
} from "lucide-react";
import { useUIStore } from "@/store/useUIStore";
import { cn } from "@/lib/utils";

interface OnboardingStep {
  id: string;
  title: string;
  description: string;
  icon: React.ElementType;
  action?: { label: string; href: string };
  tips: string[];
}

const steps: OnboardingStep[] = [
  {
    id: "welcome",
    title: "Welcome to VMOrbit",
    description:
      "VMOrbit is your unified control plane for managing virtual machines across multiple hypervisor providers. Let's get you set up in a few quick steps.",
    icon: Rocket,
    tips: [
      "VMOrbit supports VMware vCenter and Proxmox VE",
      "All operations run as async tasks you can monitor",
      "Use Ctrl+K to quickly search anything",
    ],
  },
  {
    id: "add-provider",
    title: "Connect a Provider",
    description:
      "Start by connecting your first hypervisor provider. VMOrbit will discover all your VMs, hosts, and storage automatically.",
    icon: Cpu,
    action: { label: "Go to Providers", href: "/dashboard/hypervisors" },
    tips: [
      "You'll need the provider hostname and credentials",
      "Use 'Test Connection' to verify before saving",
      "Disable TLS verify for self-signed certificates",
    ],
  },
  {
    id: "sync-inventory",
    title: "Sync Your Infrastructure",
    description:
      "After adding a provider, trigger an inventory sync to discover all your VMs, hosts, datastores, and networks.",
    icon: RefreshCw,
    action: { label: "View Tasks", href: "/dashboard/tasks" },
    tips: [
      "Sync runs in the background — watch progress in the Tasks panel",
      "Large environments may take a few minutes",
      "Set up scheduled syncs for automatic updates",
    ],
  },
  {
    id: "manage-vms",
    title: "Manage Your VMs",
    description:
      "Once synced, you can view, search, and manage all your VMs from a single interface. Power actions, snapshots, and more.",
    icon: Server,
    action: { label: "View VMs", href: "/dashboard/vms" },
    tips: [
      "Use the search bar to find VMs by name, UUID, or IP",
      "Bulk select VMs for batch operations",
      "Click a VM name for detailed info and actions",
    ],
  },
  {
    id: "shortcuts",
    title: "Pro Tips",
    description:
      "You're all set! Here are some power-user features to help you work faster.",
    icon: Keyboard,
    tips: [
      "Ctrl+K — Command palette for quick navigation",
      "Star pages in the sidebar to add them to Favorites",
      "Click the ? icon for contextual help on any page",
      "Set up automation schedules for recurring tasks",
    ],
  },
];

export function OnboardingWizard() {
  const router = useRouter();
  const onboardingComplete = useUIStore((s) => s.onboardingComplete);
  const completeOnboarding = useUIStore((s) => s.completeOnboarding);
  const [currentStep, setCurrentStep] = useState(0);

  if (onboardingComplete) return null;

  const step = steps[currentStep];
  const isLast = currentStep === steps.length - 1;

  function handleNext() {
    if (isLast) {
      completeOnboarding();
    } else {
      setCurrentStep((s) => s + 1);
    }
  }

  function handleSkip() {
    completeOnboarding();
  }

  function handleAction() {
    if (step.action) {
      router.push(step.action.href);
      completeOnboarding();
    }
  }

  return (
    <>
      {/* Backdrop */}
      <div className="fixed inset-0 bg-black/70 backdrop-blur-sm z-[200]" />

      {/* Modal */}
      <div className="fixed inset-0 flex items-center justify-center z-[201] p-4">
        <div className="bg-gray-900 border border-gray-700 rounded-2xl shadow-2xl w-full max-w-lg overflow-hidden">
          {/* Progress */}
          <div className="flex gap-1 px-6 pt-5">
            {steps.map((_, i) => (
              <div
                key={i}
                className={cn(
                  "h-1 flex-1 rounded-full transition-colors",
                  i <= currentStep ? "bg-blue-500" : "bg-gray-800"
                )}
              />
            ))}
          </div>

          {/* Content */}
          <div className="px-6 py-6">
            <div className="flex items-center gap-3 mb-4">
              <div className="w-10 h-10 rounded-xl bg-blue-600/20 border border-blue-500/30 flex items-center justify-center">
                <step.icon className="w-5 h-5 text-blue-400" />
              </div>
              <div>
                <h2 className="text-lg font-semibold text-white">{step.title}</h2>
                <p className="text-xs text-gray-500">
                  Step {currentStep + 1} of {steps.length}
                </p>
              </div>
            </div>

            <p className="text-sm text-gray-400 leading-relaxed mb-5">
              {step.description}
            </p>

            {/* Tips */}
            <div className="space-y-2 mb-6">
              {step.tips.map((tip, i) => (
                <div key={i} className="flex items-start gap-2">
                  <CheckCircle className="w-4 h-4 text-green-400 shrink-0 mt-0.5" />
                  <span className="text-sm text-gray-300">{tip}</span>
                </div>
              ))}
            </div>

            {/* Action button */}
            {step.action && (
              <button
                onClick={handleAction}
                className="w-full flex items-center justify-center gap-2 px-4 py-2.5 rounded-lg bg-blue-600/20 border border-blue-500/30 text-blue-400 text-sm font-medium hover:bg-blue-600/30 transition-colors mb-4"
              >
                <step.icon className="w-4 h-4" />
                {step.action.label}
              </button>
            )}
          </div>

          {/* Footer */}
          <div className="flex items-center justify-between px-6 py-4 border-t border-gray-800 bg-gray-900/50">
            <button
              onClick={handleSkip}
              className="text-sm text-gray-500 hover:text-gray-300 transition-colors"
            >
              Skip tour
            </button>
            <div className="flex items-center gap-2">
              {currentStep > 0 && (
                <button
                  onClick={() => setCurrentStep((s) => s - 1)}
                  className="px-4 py-2 rounded-lg text-sm text-gray-400 hover:text-white hover:bg-gray-800 transition-colors"
                >
                  Back
                </button>
              )}
              <button
                onClick={handleNext}
                className="flex items-center gap-2 px-4 py-2 rounded-lg bg-blue-600 text-white text-sm font-medium hover:bg-blue-700 transition-colors"
              >
                {isLast ? "Get Started" : "Next"}
                <ArrowRight className="w-4 h-4" />
              </button>
            </div>
          </div>
        </div>
      </div>
    </>
  );
}
