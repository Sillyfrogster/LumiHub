import { dehydrate, HydrationBoundary } from "@tanstack/react-query";
import { assetKeys, fetchAssets, makeQueryClient } from "@/lib/api/query";
import { BrowseResults } from "./BrowseResults";

export default async function BrowsePage() {
  const params = { limit: 24 };
  const queryClient = makeQueryClient();

  await queryClient.prefetchQuery({
    queryKey: assetKeys.list(params),
    queryFn: () => fetchAssets(params),
  });

  return (
    <main>
      <h1>Browse</h1>
      <HydrationBoundary state={dehydrate(queryClient)}>
        <BrowseResults params={params} />
      </HydrationBoundary>
    </main>
  );
}
