import QRCode from "qrcode";
import { useMemo } from "react";

/** A QR code drawn as one SVG path, sharp at any size and in print. */
export const QrCode = ({ text, className }: { text: string; className?: string }) => {
  const { size, d } = useMemo(() => {
    const qr = QRCode.create(text, { errorCorrectionLevel: "M" });
    const n = qr.modules.size;
    let path = "";
    for (let y = 0; y < n; y++) {
      for (let x = 0; x < n; x++) {
        if (qr.modules.get(y, x)) path += `M${x} ${y}h1v1h-1z`;
      }
    }
    return { size: n, d: path };
  }, [text]);
  return (
    <svg viewBox={`-1 -1 ${size + 2} ${size + 2}`} className={className} shapeRendering="crispEdges" aria-hidden>
      <rect x={-1} y={-1} width={size + 2} height={size + 2} fill="#fff" />
      <path d={d} fill="#000" />
    </svg>
  );
};
