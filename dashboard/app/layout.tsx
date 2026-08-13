import type { Metadata } from "next";
import "./globals.css";
import { Sidebar } from "@/components/layout/sidebar";
import { Topbar } from "@/components/layout/topbar";
import { CommandPalette } from "@/components/layout/command-palette";
import { I18nProvider } from "@/lib/i18n";

export const metadata: Metadata = {
  title: "HiveMind - Control Platform",
  description: "Distributed memory & control plane for production agent fleets",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" className="h-full" suppressHydrationWarning>
      <body className="h-screen flex bg-bg-primary text-text-primary overflow-hidden">
        {/* Apply the stored theme before first paint - no flash of wrong theme */}
        <script
          dangerouslySetInnerHTML={{
            __html: `(function(){try{var t=localStorage.getItem("hm-theme");if(t!=="light"&&t!=="dark"){t=window.matchMedia("(prefers-color-scheme: dark)").matches?"dark":"light"}document.documentElement.dataset.theme=t}catch(e){}})()`,
          }}
        />
        <I18nProvider>
          <Sidebar />
          <div className="flex-1 flex flex-col min-w-0">
            <Topbar />
            <main className="flex-1 overflow-y-auto">{children}</main>
          </div>
          <CommandPalette />
        </I18nProvider>
      </body>
    </html>
  );
}
