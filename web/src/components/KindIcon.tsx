import { Shapes } from "lucide-react";
import { customIconUrl } from "@/lib/api";
import { ICONS } from "@/lib/icons";
import { cn } from "@/lib/utils";

/**
 * A kind's icon. Custom icons are drawn as a mask, so they take the text
 * colour like the built-in ones.
 */
export const KindIcon = ({ icon, className }: { icon: string; className?: string }) => {
  if (icon.startsWith("custom:")) {
    const url = `url("${customIconUrl(icon)}")`;
    return (
      <span
        aria-hidden
        className={cn("inline-block size-4 shrink-0 bg-current", className)}
        style={{
          maskImage: url,
          WebkitMaskImage: url,
          maskSize: "contain",
          WebkitMaskSize: "contain",
          maskRepeat: "no-repeat",
          WebkitMaskRepeat: "no-repeat",
          maskPosition: "center",
          WebkitMaskPosition: "center",
        }}
      />
    );
  }
  const Icon = ICONS[icon] ?? Shapes;
  return <Icon className={className} aria-hidden />;
};
