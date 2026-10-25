import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api';
import type { CampaignInput, CampaignStatsQuery, CreateLinkInChannelInput } from '@/types';
import { queryKeys, useActiveGroupId } from './keys';

/** Campaigns of the active group (with channel/link counts and all-time engagements). */
export function useCampaigns() {
  const groupId = useActiveGroupId();
  return useQuery({
    queryKey: queryKeys.campaigns(groupId),
    queryFn: async () => (await api.listCampaigns()).campaigns,
    enabled: groupId != null,
  });
}

export function useCampaign(id: number | null | undefined) {
  return useQuery({
    queryKey: queryKeys.campaign(id ?? 0),
    queryFn: () => api.getCampaign(id as number),
    enabled: id != null && id > 0,
  });
}

export function useCampaignStats(id: number | null | undefined, query: CampaignStatsQuery = {}) {
  return useQuery({
    queryKey: queryKeys.campaignStats(id ?? 0, query),
    queryFn: () => api.getCampaignStats(id as number, query),
    enabled: id != null && id > 0,
    placeholderData: keepPreviousData,
  });
}

/** Channels of the active group (shared by its campaigns). */
export function useChannels() {
  const groupId = useActiveGroupId();
  return useQuery({
    queryKey: queryKeys.channels(groupId),
    queryFn: async () => (await api.listChannels()).channels,
    enabled: groupId != null,
  });
}

/** The campaigns/channels a link belongs to. */
export function useLinkCampaigns(linkId: number | null | undefined) {
  return useQuery({
    queryKey: queryKeys.linkCampaigns(linkId ?? 0),
    queryFn: async () => (await api.getLinkCampaigns(linkId as number)).memberships,
    enabled: linkId != null && linkId > 0,
  });
}

/**
 * Campaign, channel and membership mutations. Each invalidates the campaign
 * list and the touched campaign (detail + stats); membership changes also
 * invalidate links (list filter `campaign_id`, link memberships).
 */
export function useCampaignMutations() {
  const qc = useQueryClient();
  const invalidate = (id?: number, { links = false, channels = false } = {}) => {
    qc.invalidateQueries({ queryKey: ['campaigns'] });
    if (id != null) qc.invalidateQueries({ queryKey: queryKeys.campaign(id) });
    if (channels) qc.invalidateQueries({ queryKey: ['channels'] });
    if (links) {
      qc.invalidateQueries({ queryKey: ['links'] });
      qc.invalidateQueries({ queryKey: ['link'] });
    }
  };

  const create = useMutation({
    mutationFn: (input: CampaignInput) => api.createCampaign(input),
    onSuccess: (campaign) => {
      qc.setQueryData(queryKeys.campaign(campaign.id), campaign);
      invalidate(undefined, { channels: true });
    },
  });

  const update = useMutation({
    mutationFn: ({ id, input }: { id: number; input: CampaignInput }) => api.updateCampaign(id, input),
    onSuccess: (campaign, { id }) => {
      qc.setQueryData(queryKeys.campaign(id), campaign);
      invalidate(id, { channels: true, links: true });
    },
  });

  const remove = useMutation({
    mutationFn: (id: number) => api.deleteCampaign(id),
    onSuccess: (_data, id) => {
      qc.removeQueries({ queryKey: queryKeys.campaign(id) });
      invalidate(undefined, { links: true });
    },
  });

  const addLinks = useMutation({
    mutationFn: ({ campaignId, channelId, linkIds }: { campaignId: number; channelId: number; linkIds: number[] }) =>
      api.addCampaignLinks(campaignId, channelId, linkIds),
    onSuccess: (_data, { campaignId }) => invalidate(campaignId, { links: true }),
  });

  const removeLink = useMutation({
    mutationFn: ({ campaignId, channelId, linkId }: { campaignId: number; channelId: number; linkId: number }) =>
      api.removeCampaignLink(campaignId, channelId, linkId),
    onSuccess: (_data, { campaignId }) => invalidate(campaignId, { links: true }),
  });

  const createLinkInChannel = useMutation({
    mutationFn: ({
      campaignId,
      channelId,
      input,
    }: {
      campaignId: number;
      channelId: number;
      input: CreateLinkInChannelInput;
    }) => api.createLinkInChannel(campaignId, channelId, input),
    onSuccess: (_link, { campaignId }) => invalidate(campaignId, { links: true }),
  });

  const createChannel = useMutation({
    mutationFn: (name: string) => api.createChannel(name),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['channels'] }),
  });

  return { create, update, remove, addLinks, removeLink, createLinkInChannel, createChannel };
}
