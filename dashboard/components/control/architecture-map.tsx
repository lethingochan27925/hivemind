"use client";

import { ReactNode } from "react";
import Link from "next/link";
import { api, LambdaInfo, ResourceInfo } from "@/lib/api";
import { useLive } from "@/lib/use-live";
import {
  Globe,
  HardDrive,
  Server,
  Cpu,
  Database,
  Brain,
  CalendarClock,
  Activity,
  KeyRound,
  Package,
  Bell,
  Table2,
} from "lucide-react";

const ACCENT: Record<string, string> = {
  green: "bg-green",
  blue: "bg-blue",
  purple: "bg-purple",
  yellow: "bg-yellow",
  red: "bg-red",
  teal: "bg-teal",
  orange: "bg-orange",
};

function stateTone(s: string) {
  return s === "Active" ? "green" : s === "Pending" ? "yellow" : "red";
}

function Node({
  icon,
  title,
  sub,
  accent = "blue",
  live,
  href,
}: {
  icon: ReactNode;
  title: string;
  sub?: string;
  accent?: string;
  live?: boolean;
  href?: string;
}) {
  const card = (
    <div className={`relative flex flex-col gap-1.5 min-w-[150px] rounded-xl border border-border bg-bg-panel px-4 py-3 shadow-[var(--node-shadow)] ${href ? "hover:border-border-strong transition-colors" : ""}`}>
      <span className={`absolute left-0 top-2 bottom-2 w-1 rounded-full ${ACCENT[accent]}`} />
      <div className="flex items-center gap-2">
        <span className="text-text-secondary">{icon}</span>
        <span className="text-[14px] font-semibold text-text-primary leading-none">{title}</span>
        {live && <span className="ml-auto w-2 h-2 rounded-full bg-green hm-pulse" />}
      </div>
      {sub && <span className="text-[12px] text-text-tertiary tnum">{sub}</span>}
    </div>
  );
  // Moi node deu dan sang dung cho control tuong ung trong Infrastructure.
  return href ? (
    <Link href={href} className="block">
      {card}
    </Link>
  ) : (
    card
  );
}

function Tier({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="relative">
      <div className="text-[11px] uppercase tracking-[0.14em] text-text-tertiary font-bold mb-2.5">
        {label}
      </div>
      <div className="flex flex-wrap gap-3">{children}</div>
    </div>
  );
}

// Spine - the connector between tiers, with a downward-flowing pulse.
function Spine() {
  return (
    <div className="relative h-7 flex justify-center">
      <div className="w-px h-full bg-gradient-to-b from-border-strong to-border" />
      <span className="absolute left-1/2 -translate-x-1/2 w-1.5 h-1.5 rounded-full bg-blue hm-pulse" />
    </div>
  );
}

/**
 * ArchitectureMap - a live, self-assembling system map. Nodes are generated from
 * the real deployed infrastructure (/control/lambdas, /control/db, /control/resources)
 * and grouped into architectural tiers; live state (Lambda health, DB row counts,
 * schedule/registry/observability counts) is overlaid. Not a static drawing.
 */
export function ArchitectureMap() {
  const lambdas = useLive(api.getLambdas, 20000);
  const db = useLive(api.getDbStats, 20000);
  const resources = useLive(api.getResources, 60000);

  const fns = lambdas.data ?? [];
  const byName = (n: string): LambdaInfo | undefined => fns.find((f) => f.service === n);
  const fleet = fns.filter((f) => f.service !== "dashboard-api");

  const res = resources.data ?? [];
  const count = (svc: string) => res.filter((r: ResourceInfo) => r.service === svc).length;

  const dbTables = db.data?.tables ?? [];
  const totalRows = db.data?.total_rows ?? 0;

  const fnAccent: Record<string, string> = {
    "agent-worker": "blue",
    dispatcher: "teal",
    reaper: "orange",
    "salience-decay": "purple",
    "scoring-api": "green",
    "scoring-python": "green",
  };

  return (
    <div className="rounded-xl border border-border bg-bg-inset/60 p-5 bg-[radial-gradient(circle_at_1px_1px,rgba(255,255,255,0.04)_1px,transparent_0)] bg-[length:22px_22px]">
      <div className="space-y-1">
        {/* Edge */}
        <Tier label="Edge · Delivery">
          <Node
            icon={<Globe size={16} />}
            title="CloudFront"
            sub={`${count("cloudfront")} distribution`}
            accent="blue"
            href="/infrastructure?res=cloudfront"
            live
          />
          <Node
            icon={<HardDrive size={16} />}
            title="S3"
            sub={`${count("s3")} buckets · static site`}
            accent="teal"
            href="/infrastructure?res=s3"
          />
        </Tier>

        <Spine />

        {/* API */}
        <Tier label="Control plane · API">
          <Node
            icon={<Server size={16} />}
            title="dashboard-api"
            sub={
              byName("dashboard-api")
                ? `Lambda · v${byName("dashboard-api")!.version} · ${byName("dashboard-api")!.memory_mb}MB`
                : "Lambda Function URL"
            }
            accent={byName("dashboard-api") ? stateTone(byName("dashboard-api")!.state) : "blue"}
            href="/infrastructure?node=dashboard-api"
            live={byName("dashboard-api")?.state === "Active"}
          />
        </Tier>

        <Spine />

        {/* Compute fleet */}
        <Tier label={`Agent fleet · Compute (${fleet.length} functions)`}>
          {fleet.length === 0 ? (
            <span className="text-[13px] text-text-tertiary">Loading fleet…</span>
          ) : (
            fleet.map((f) => (
              <Node
                key={f.service}
                href={`/infrastructure?node=${f.service}`}
                icon={<Cpu size={16} />}
                title={f.service}
                sub={`v${f.version} · ${f.memory_mb}MB · ${f.timeout_sec}s`}
                accent={fnAccent[f.service] ?? "blue"}
                live={f.state === "Active"}
              />
            ))
          )}
        </Tier>

        <Spine />

        {/* State + AI */}
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          <div>
            <div className="text-[11px] uppercase tracking-[0.14em] text-text-tertiary font-bold mb-2.5">
              State · CockroachDB
            </div>
            <Link href="/database" className="block rounded-xl border border-border bg-bg-panel px-4 py-3 hover:border-border-strong transition-colors">
              <div className="flex items-center gap-2 mb-2.5">
                <Database size={16} className="text-purple" />
                <span className="text-[14px] font-semibold text-text-primary">
                  {db.data?.database ?? "hivemind"}
                </span>
                <span className="ml-auto text-[12px] text-text-tertiary tnum">
                  {totalRows.toLocaleString()} rows
                </span>
              </div>
              <div className="grid grid-cols-2 gap-2">
                {dbTables.map((t) => (
                  <div
                    key={t.table}
                    className="flex items-center justify-between rounded-lg bg-bg-inset border border-border px-2.5 py-1.5"
                  >
                    <span className="text-[12px] text-text-secondary">{t.table}</span>
                    <span className="tnum text-[13px] font-semibold text-text-primary">
                      {t.rows.toLocaleString()}
                    </span>
                  </div>
                ))}
              </div>
            </Link>
          </div>

          <div>
            <div className="text-[11px] uppercase tracking-[0.14em] text-text-tertiary font-bold mb-2.5">
              AI · Amazon Bedrock
            </div>
            <div className="flex flex-wrap gap-3">
              <Node icon={<Brain size={16} />} title="Claude Haiku" sub="reasoning · verdicts" accent="orange" href="/cost" />
              <Node icon={<Brain size={16} />} title="Titan Embeddings" sub="1024-dim vectors" accent="teal" href="/cost" />
            </div>
          </div>
        </div>

        <Spine />

        {/* Platform services */}
        <Tier label="Platform services">
          <Node icon={<CalendarClock size={16} />} title="EventBridge" sub={`${count("events")} schedules`} accent="yellow" href="/infrastructure?res=events" />
          <Node icon={<Activity size={16} />} title="CloudWatch" sub={`${count("cloudwatch")} alarms · ${count("logs")} log groups`} accent="green" href="/infrastructure?res=cloudwatch" />
          <Node icon={<KeyRound size={16} />} title="SSM" sub={`${count("ssm")} parameters`} accent="blue" href="/infrastructure?res=ssm" />
          <Node icon={<Package size={16} />} title="ECR" sub={`${count("ecr")} repositories`} accent="purple" href="/infrastructure?res=ecr" />
          <Node icon={<Bell size={16} />} title="SNS" sub={`${count("sns")} topic`} accent="red" href="/infrastructure?res=sns" />
          <Node icon={<Table2 size={16} />} title="DynamoDB" sub={`${count("dynamodb")} tfstate lock`} accent="teal" href="/infrastructure?res=dynamodb" />
        </Tier>
      </div>
    </div>
  );
}
