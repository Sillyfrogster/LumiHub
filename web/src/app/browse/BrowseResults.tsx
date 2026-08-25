"use client";

import { useQuery } from "@tanstack/react-query";
import { type AssetListParams, assetKeys, fetchAssets } from "@/lib/api/query";

export function BrowseResults({ params }: { params: AssetListParams }) {
  const { data, isPending, isError } = useQuery({
    queryKey: assetKeys.list(params),
    queryFn: () => fetchAssets(params),
  });

  if (isPending) return <p>Loading assets.</p>;
  if (isError) return <p>Could not load assets. Try again shortly.</p>;
  if (data.length === 0) return <p>Nothing here yet.</p>;

  return (
    <ul>
      {data.map((asset) => (
        <li key={asset.id}>
          <strong>{asset.name}</strong> <span>{asset.kind}</span>
        </li>
      ))}
    </ul>
  );
}
