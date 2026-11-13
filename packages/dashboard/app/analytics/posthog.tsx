"use client";
/**
 * PostHog provider for the Vigil dashboard.
 * - autocapture: OFF (explicit events only via the generated typed client)
 * - session_recording: ON with maskAllInputs=true (no keystrokes/addresses recorded)
 * - identify on login (user_id = UUID, not email); group by org_id
 * - reset on logout
 */

import { useEffect } from "react";
import type { PostHog } from "posthog-js";

let ph: PostHog | null = null;

export function initPostHog(): void {
  if (typeof window === "undefined") return;
  if (ph !== null) return;
  const key = process.env["NEXT_PUBLIC_POSTHOG_KEY"];
  if (!key) return;

  import("posthog-js").then(({ default: posthog }) => {
    posthog.init(key, {
      api_host: process.env["NEXT_PUBLIC_POSTHOG_HOST"] ?? "https://app.posthog.com",
      autocapture: false,
      capture_pageview: false, // manual page views via trackPageView()
      capture_pageleave: true,
      session_recording: {
        maskAllInputs: true,       // mask text inputs (addresses, keys, etc.)
        maskTextSelector: "[data-mask]",
      },
      opt_out_capturing_by_default: false,
      persistence: "localStorage+cookie",
    });
    ph = posthog;
  });
}

/** Call on login: stitch anonymous_id to user_id, group by org. */
export function identifyUser(userId: string, orgId: string, plan: string): void {
  if (!ph) return;
  ph.identify(userId);
  ph.group("org", orgId, { plan });
}

/** Call on logout: reset all identity. */
export function resetIdentity(): void {
  if (!ph) return;
  ph.reset();
}

/** Fire an explicit page view. Called from each page component. */
export function trackPageView(pageName: string, loadTimeMs?: number): void {
  if (!ph) return;
  ph.capture("dashboard_page_viewed", {
    page_name: pageName,
    load_time_ms: loadTimeMs ?? 0,
    surface: "web",
  });
}

/** Generic typed event proxy — use generated tracking.ts instead when possible. */
export function trackEvent(event: string, props: Record<string, unknown>): void {
  if (!ph) return;
  ph.capture(event, { ...props, surface: "web" });
}

export function PostHogInit(): null {
  useEffect(() => { initPostHog(); }, []);
  return null;
}
