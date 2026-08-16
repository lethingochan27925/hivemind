"use client";

import { api, InfrastructureData } from "@/lib/api";
import { createSharedPoll } from "@/lib/use-shared-poll";

// Shared by the topbar's alarm chip and the Infrastructure page's own panels -
// both read api.getInfrastructure(), so they now share one poll instead of
// running it twice at two different intervals (20s in the topbar, 5s on the
// page). 5s keeps the page's original freshness; the topbar chip simply gets
// it for free.
const infrastructurePoll = createSharedPoll<InfrastructureData>(api.getInfrastructure, 5000);

export const useInfrastructure = infrastructurePoll.use;
export const refreshInfrastructure = infrastructurePoll.refresh;
