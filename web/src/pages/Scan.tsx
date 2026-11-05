import jsQR from "jsqr";
import { CameraOff, ScanLine } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card } from "@/components/ui/misc";
import { codeFrom } from "@/lib/labels";

interface Detector {
  detect: (source: CanvasImageSource) => Promise<{ rawValue: string }[]>;
}
declare global {
  interface Window {
    BarcodeDetector?: new (options: { formats: string[] }) => Detector;
  }
}

/** Scans a label's QR code with the camera, or takes a typed code. */
export const Scan = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const videoRef = useRef<HTMLVideoElement>(null);
  const [error, setError] = useState<string | null>(null);
  const [typed, setTyped] = useState("");

  useEffect(() => {
    let stream: MediaStream | null = null;
    let stopped = false;
    let frame = 0;
    const canvas = document.createElement("canvas");
    const detector = window.BarcodeDetector ? new window.BarcodeDetector({ formats: ["qr_code"] }) : null;

    const found = (text: string) => {
      const code = codeFrom(text);
      if (!code) return false;
      stopped = true;
      void navigate(`/l/${code}`, { replace: true });
      return true;
    };

    // The browser's own detector where there is one (Chrome, Android),
    // jsQR elsewhere (Safari).
    const tick = async () => {
      if (stopped) return;
      const v = videoRef.current;
      if (v && v.readyState >= 2) {
        try {
          if (detector) {
            const codes = await detector.detect(v);
            if (codes.some((c) => found(c.rawValue))) return;
          } else {
            canvas.width = v.videoWidth;
            canvas.height = v.videoHeight;
            const ctx = canvas.getContext("2d", { willReadFrequently: true });
            if (ctx) {
              ctx.drawImage(v, 0, 0);
              const img = ctx.getImageData(0, 0, canvas.width, canvas.height);
              const qr = jsQR(img.data, img.width, img.height, { inversionAttempts: "dontInvert" });
              if (qr && found(qr.data)) return;
            }
          }
        } catch {
          /* a frame that couldn't be read */
        }
      }
      frame = requestAnimationFrame(() => void tick());
    };

    navigator.mediaDevices
      .getUserMedia({ video: { facingMode: "environment" }, audio: false })
      .then((s) => {
        if (stopped) {
          for (const tr of s.getTracks()) tr.stop();
          return;
        }
        stream = s;
        if (videoRef.current) {
          videoRef.current.srcObject = s;
          void videoRef.current.play();
        }
        void tick();
      })
      .catch(() => setError(t("scan.noCamera")));

    return () => {
      stopped = true;
      cancelAnimationFrame(frame);
      for (const tr of stream?.getTracks() ?? []) tr.stop();
    };
  }, [navigate, t]);

  const code = codeFrom(typed);
  return (
    <div className="animate-fade-in mx-auto flex max-w-md flex-col gap-5">
      <h1 className="text-2xl font-semibold tracking-tight">{t("scan.title")}</h1>
      {error ? (
        <Card className="flex flex-col items-center gap-2 px-6 py-10 text-center">
          <CameraOff className="size-6 text-fg-subtle" />
          <p className="text-sm text-fg-muted">{error}</p>
        </Card>
      ) : (
        <div className="relative aspect-square overflow-hidden rounded-2xl bg-black">
          <video ref={videoRef} muted playsInline className="size-full object-cover" />
          <div className="pointer-events-none absolute inset-[15%] rounded-2xl border-2 border-white/70" />
          <ScanLine className="pointer-events-none absolute top-1/2 left-1/2 size-10 -translate-1/2 text-white/60" />
        </div>
      )}
      <p className="text-sm text-fg-muted">{t("scan.hint")}</p>
      <form
        className="flex gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          if (code) void navigate(`/l/${code}`);
        }}
      >
        <Input
          value={typed}
          onChange={(e) => setTyped(e.target.value)}
          placeholder={t("scan.typeCode")}
          className="font-mono uppercase"
          autoCapitalize="characters"
        />
        <Button type="submit" disabled={!code}>
          {t("scan.open")}
        </Button>
      </form>
    </div>
  );
};
