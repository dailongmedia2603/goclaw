import { useNavigate, useParams } from "react-router";
import { useTranslation } from "react-i18next";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { PageHeader } from "@/components/shared/page-header";
import { CredentialsTab } from "./credentials-tab";
import { JobsTab } from "./jobs-tab";
import { PlansTab } from "./plans-tab";
import { SendLogTab } from "./send-log-tab";

const VALID_TABS = ["credentials", "jobs", "plans", "log"] as const;
type FbcloakTab = (typeof VALID_TABS)[number];

function isValidTab(v: string | undefined): v is FbcloakTab {
  return !!v && (VALID_TABS as readonly string[]).includes(v);
}

export function FBCloakPage() {
  const { t } = useTranslation("fbcloak");
  const navigate = useNavigate();
  const { tab: rawTab } = useParams<{ tab?: string }>();
  const tab: FbcloakTab = isValidTab(rawTab) ? rawTab : "credentials";

  return (
    <div className="p-4 sm:p-6 pb-10">
      <PageHeader title={t("title")} description={t("description")} />

      <Tabs
        value={tab}
        onValueChange={(v) => navigate(`/fbcloak/${v}`)}
        className="mt-4"
      >
        <TabsList className="overflow-x-auto">
          <TabsTrigger value="credentials">{t("tabs.credentials")}</TabsTrigger>
          <TabsTrigger value="jobs">{t("tabs.jobs")}</TabsTrigger>
          <TabsTrigger value="plans">{t("tabs.plans")}</TabsTrigger>
          <TabsTrigger value="log">{t("tabs.sendLog")}</TabsTrigger>
        </TabsList>

        <TabsContent value="credentials">
          <CredentialsTab />
        </TabsContent>
        <TabsContent value="jobs">
          <JobsTab />
        </TabsContent>
        <TabsContent value="plans">
          <PlansTab />
        </TabsContent>
        <TabsContent value="log">
          <SendLogTab />
        </TabsContent>
      </Tabs>
    </div>
  );
}
