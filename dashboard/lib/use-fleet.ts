"use client";

import { api, FleetStatus } from "@/lib/api";
import { createSharedPoll, SharedPollSnapshot } from "@/lib/use-shared-poll";

/**
 * useFleet - ONE poll of the fleet state, shared by every component that shows it.
 *
 * The topbar chip, the Mission Control banner, the fleet command bar and the
 * infrastructure page all display "is the fleet running". Each used to poll on
 * its own interval (8s, 4s, 10s), so the same fact could be rendered three
 * different ways at once - the chip said PAUSED while the panel said running -
 * and one open tab cost three times the Lambda invocations it needed to.
 *
 * Built on lib/use-shared-poll's createSharedPoll, the same reference-counted,
 * tab-visibility-aware primitive this hook was the original (hand-rolled)
 * template for - infra/cost's shared hooks were generalised from this file, so
 * this file is now built on that generalisation rather than carrying its own
 * separate copy of the same start-on-first-subscriber/stop-on-last machinery.
 */
export type FleetSnapshot = SharedPollSnapshot<FleetStatus>;

const fleetPoll = createSharedPoll<FleetStatus>(api.getFleetStatus, 4000);

export const useFleet = fleetPoll.use;

/**
 * refreshFleet - poll immediately instead of waiting out the interval. Called
 * after an action so the UI reflects it at once; without this, pressing "Start
 * fleet" left the badge reading "paused" for up to four seconds and people
 * pressed it again.
 */
export const refreshFleet = fleetPoll.refresh;
