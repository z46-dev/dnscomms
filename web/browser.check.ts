import { expect, test } from "@playwright/test";

const dohQueries: string[] = [];

// The browser exercises Go's real DoH request path with deterministic DNS replies in CI.
test.beforeEach(async ({ page }) => {
    dohQueries.length = 0;
    await page.route(
        "https://cloudflare-dns.com/dns-query?*",
        async (route) => {
            dohQueries.push(route.request().url());
            const wire = Buffer.from(
                new URL(route.request().url()).searchParams.get("dns") ?? "",
                "base64url"
            );
            wire[2] = (wire[2] ?? 0) | 0x80;
            wire[3] = (wire[3] ?? 0) | 0x80;
            await route.fulfill({
                status: 200,
                contentType: "application/dns-message",
                body: wire,
                headers: { "access-control-allow-origin": "*" }
            });
        }
    );
});

test("combined exfil, configuration, capture and vertical map work together", async ({
    page
}) => {
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    await page.goto("/");
    await expect(
        page.getByRole("button", { name: "Send transfer" })
    ).toBeEnabled();
    await expect(page.getByRole("tab")).toHaveText([
        "DNS Exfil",
        "Packet capture",
        "Config"
    ]);
    await page
        .getByRole("textbox", { name: "Message", exact: true })
        .fill("Browser round trip 世界");
    await page.getByLabel("Duration (seconds)").fill("1");
    await page.getByRole("button", { name: "Send transfer" }).click();
    await expect(
        page
            .getByRole("region", { name: "Orchestrator inbox" })
            .getByText("Browser round trip 世界", { exact: true })
    ).toBeVisible();
    await page.getByRole("tab", { name: "Config", exact: true }).click();
    await page.getByLabel("DNS servers (total)").selectOption("6");
    await page
        .getByRole("combobox", { name: "Exfil servers", exact: true })
        .selectOption("3");
    await expect(page.getByLabel("dns-6 is an exfil server")).toBeVisible();
    await page.getByLabel("Regular DNS on dns-1").selectOption("dns-4");
    await page.getByLabel("Regular DNS frequency").fill("8");
    await expect(page.getByText("8.00 queries/sec")).toBeVisible();
    await page.getByLabel("Allow regular DNS from the exfil client").check();
    await page.getByRole("tab", { name: "DNS Exfil", exact: true }).focus();
    await page.keyboard.press("ArrowRight");
    await expect(
        page.getByRole("tab", { name: "Packet capture" })
    ).toBeFocused();
    await expect(
        page.getByRole("tab", { name: "Packet capture" })
    ).toHaveAttribute("aria-selected", "true");
    await page.getByText("Filters", { exact: true }).click();
    await page
        .getByRole("combobox", { name: "classification", exact: true })
        .selectOption("exfil part");
    await page
        .getByRole("button", { name: /^Inspect event / })
        .first()
        .click();
    await expect(page.getByText("Exfil header", { exact: true })).toBeVisible();
    await page
        .getByRole("combobox", { name: "classification", exact: true })
        .selectOption("forwarded part");
    await expect(
        page.getByRole("table").getByText("forwarded part")
    ).toBeVisible();
    await page.getByRole("button", { name: "Clear filters" }).click();
    await page.getByLabel("Search packets").fill("dns-4");
    await expect(
        page
            .getByRole("table")
            .getByText("dns-1 → dns-4", { exact: true })
            .first()
    ).toBeVisible({ timeout: 10000 });
    await expect.poll(() => dohQueries.length).toBeGreaterThan(0);
    await page.getByText(/^Filters/).click();
    await page.getByRole("button", { name: "Pause", exact: true }).click();
    const paused = await page.getByRole("status").innerText();
    await page.waitForTimeout(250);
    await expect(page.getByRole("status")).toHaveText(paused);
    await page.setViewportSize({ width: 375, height: 812 });
    await page.emulateMedia({ reducedMotion: "reduce" });
    expect(
        await page.evaluate(
            () => document.documentElement.scrollWidth <= innerWidth
        )
    ).toBe(true);
    const map = page.locator("canvas");
    await expect(map).toBeVisible();
    expect(
        await map.evaluate((canvas) => {
            const surface = canvas as HTMLCanvasElement;
            return (
                surface
                    .getContext("2d")
                    ?.getImageData(0, 0, surface.width, surface.height)
                    .data.some((value) => value !== 0) ?? false
            );
        })
    ).toBe(true);
    await page.screenshot({ path: "/tmp/dnscomms-mobile.png", fullPage: true });
    await page.getByRole("button", { name: "Reset", exact: true }).click();
    await expect(page.getByText("Exfil header", { exact: true })).toHaveCount(
        0
    );
    await page.getByRole("tab", { name: "DNS Exfil", exact: true }).click();
    await page.getByLabel("dns-1exfil", { exact: true }).uncheck();
    await page.getByLabel("dns-2normal", { exact: true }).check();
    await page.getByRole("button", { name: "Send transfer" }).click();
    await expect(page.getByText("incomplete", { exact: true })).toBeVisible();
    await expect(
        page
            .getByRole("region", { name: "Orchestrator inbox" })
            .getByText("Browser round trip 世界", { exact: true })
    ).toHaveCount(0);
    await page.setViewportSize({ width: 1400, height: 1000 });
    await page.screenshot({
        path: "/tmp/dnscomms-desktop.png",
        fullPage: true
    });
    expect(errors).toEqual([]);
});
