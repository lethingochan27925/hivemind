"use client";

import { api, ResourceInfo } from "@/lib/api";
import { useLive } from "@/lib/use-live";
import { Panel } from "@/components/ui/panel";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/ui/empty-state";
import { PageHeader } from "@/components/ui/page-header";
import { ArchitectureMap } from "@/components/control/architecture-map";

const SERVICE_LABEL: Record<string, string> = {
  lambda: "Lambda functions",
  cloudwatch: "CloudWatch alarms",
  logs: "Log groups",
  ssm: "SSM parameters",
  ecr: "ECR repositories",
  s3: "S3 buckets",
  events: "EventBridge rules",
  dynamodb: "DynamoDB tables",
  sns: "SNS topics",
  cloudfront: "CloudFront",
  iam: "IAM roles",
};

export default function ArchitecturePage() {
  const { data, error, lastUpdated } = useLive(api.getResources, 60000);
  const resources = data ?? [];

  const grouped = resources.reduce<Record<string, ResourceInfo[]>>((acc, r) => {
    (acc[r.service] ??= []).push(r);
    return acc;
  }, {});
  const services = Object.keys(grouped).sort((a, b) => grouped[b].length - grouped[a].length);

  return (
    <div>
      <PageHeader
        title="System Architecture"
        description="Live topology assembled from the deployed infrastructure - every node is a real resource"
        lastUpdated={lastUpdated}
        error={error}
        actions={
          resources.length > 0 ? (
            <Badge variant="blue" dot>
              {resources.length} resources · {services.length} services
            </Badge>
          ) : null
        }
      />

      <div className="p-6 space-y-5 hm-enter">
        <ArchitectureMap />

        <Panel
          title="Infrastructure inventory"
          subtitle="every resource Terraform tagged Project=hivemind"
        >
          {services.length === 0 ? (
            <EmptyState message="Loading inventory…" />
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
              {services.map((svc) => (
                <div key={svc} className="rounded-lg border border-border bg-bg-inset/40 overflow-hidden">
                  <div className="flex items-center justify-between px-3.5 h-10 border-b border-border bg-bg-inset/60">
                    <span className="text-[13px] font-semibold text-text-primary">
                      {SERVICE_LABEL[svc] ?? svc}
                    </span>
                    <span className="tnum text-[13px] text-text-tertiary">{grouped[svc].length}</span>
                  </div>
                  <ul className="px-3.5 py-2 space-y-1 max-h-44 overflow-y-auto">
                    {grouped[svc].map((r) => (
                      <li key={r.arn} className="text-[12px] text-text-secondary tnum truncate">
                        {r.name}
                      </li>
                    ))}
                  </ul>
                </div>
              ))}
            </div>
          )}
        </Panel>
      </div>
    </div>
  );
}
