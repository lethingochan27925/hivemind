"use client";

import { api, CostData } from "@/lib/api";
import { createSharedPoll } from "@/lib/use-shared-poll";

// Shared by the topbar's spend chip and the Cost page's own panels - both
// read api.getCost(), so they now share one poll instead of running it twice
// at two different intervals (60s in the topbar, 10s on the page). 10s keeps
// the page's original freshness; the topbar chip simply gets it for free.
const costPoll = createSharedPoll<CostData>(api.getCost, 10000);

export const useCost = costPoll.use;
export const refreshCost = costPoll.refresh;
