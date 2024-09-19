// Copyright (C) The Arvados Authors. All rights reserved.
//
// SPDX-License-Identifier: AGPL-3.0

import { ServiceRepository } from 'services/services';
import { MiddlewareAPI, Dispatch } from 'redux';
import { DataExplorerMiddlewareService, dataExplorerToListParams, getOrder, listResultsToDataExplorerItemsMeta } from 'store/data-explorer/data-explorer-middleware-service';
import { RootState } from 'store/store';
import { snackbarActions, SnackbarKind } from 'store/snackbar/snackbar-actions';
import { DataExplorer, getDataExplorer } from 'store/data-explorer/data-explorer-reducer';
import { updateResources } from 'store/resources/resources-actions';
import { ListArguments, ListResults } from 'services/common-service/common-service';
import { LinkResource } from 'models/link';
import { linkPanelActions } from 'store/link-panel/link-panel-actions';
import { progressIndicatorActions } from "store/progress-indicator/progress-indicator-actions";
import { couldNotFetchItemsAvailable } from 'store/data-explorer/data-explorer-action';

export class LinkMiddlewareService extends DataExplorerMiddlewareService {
    constructor(private services: ServiceRepository, id: string) {
        super(id);
    }

    async requestItems(api: MiddlewareAPI<Dispatch, RootState>, criteriaChanged?: boolean, background?: boolean) {
        const state = api.getState();
        const dataExplorer = getDataExplorer(state.dataExplorer, this.getId());
        try {
            if (!background) { api.dispatch(progressIndicatorActions.START_WORKING(this.getId())); }
            const response = await this.services.linkService.list(getParams(dataExplorer));
            api.dispatch(updateResources(response.items));
            api.dispatch(setItems(response));
        } catch {
            api.dispatch(couldNotFetchLinks());
        } finally {
            api.dispatch(progressIndicatorActions.STOP_WORKING(this.getId()));
        }
    }

    async requestCount(api: MiddlewareAPI<Dispatch, RootState>, criteriaChanged?: boolean, background?: boolean) {
        if (criteriaChanged) {
            // Get itemsAvailable
            return this.services.linkService.list(getCountParams())
                .then((results: ListResults<LinkResource>) => {
                    if (results.itemsAvailable !== undefined) {
                        api.dispatch<any>(linkPanelActions.SET_ITEMS_AVAILABLE(results.itemsAvailable));
                    } else {
                        couldNotFetchItemsAvailable();
                    }
                });
        }
    }
}

export const getParams = (dataExplorer: DataExplorer): ListArguments => ({
    ...dataExplorerToListParams(dataExplorer),
    order: getOrder<LinkResource>(dataExplorer),
    count: 'none',
});

const getCountParams = (): ListArguments => ({
    limit: 0,
    count: 'exact',
});

export const setItems = (listResults: ListResults<LinkResource>) =>
    linkPanelActions.SET_ITEMS({
        ...listResultsToDataExplorerItemsMeta(listResults),
        items: listResults.items.map(resource => resource.uuid),
    });

const couldNotFetchLinks = () =>
    snackbarActions.OPEN_SNACKBAR({
        message: 'Could not fetch links.',
        kind: SnackbarKind.ERROR
    });
