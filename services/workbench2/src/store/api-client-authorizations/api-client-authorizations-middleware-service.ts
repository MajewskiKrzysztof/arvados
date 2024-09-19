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
import { apiClientAuthorizationsActions } from 'store/api-client-authorizations/api-client-authorizations-actions';
import { ListArguments, ListResults } from 'services/common-service/common-service';
import { ApiClientAuthorization } from 'models/api-client-authorization';
import { couldNotFetchItemsAvailable } from 'store/data-explorer/data-explorer-action';

export class ApiClientAuthorizationMiddlewareService extends DataExplorerMiddlewareService {
    constructor(private services: ServiceRepository, id: string) {
        super(id);
    }

    async requestItems(api: MiddlewareAPI<Dispatch, RootState>) {
        const state = api.getState();
        const dataExplorer = getDataExplorer(state.dataExplorer, this.getId());
        try {
            const response = await this.services.apiClientAuthorizationService.list(getParams(dataExplorer));
            api.dispatch(updateResources(response.items));
            api.dispatch(setItems(response));
        } catch {
            api.dispatch(couldNotFetchLinks());
        }
    }

    async requestCount(api: MiddlewareAPI<Dispatch, RootState>, criteriaChanged?: boolean, background?: boolean) {
        if (criteriaChanged) {
            // Get itemsAvailable
            return this.services.apiClientAuthorizationService.list(getCountParams())
                .then((results: ListResults<ApiClientAuthorization>) => {
                    if (results.itemsAvailable !== undefined) {
                        api.dispatch<any>(apiClientAuthorizationsActions.SET_ITEMS_AVAILABLE(results.itemsAvailable));
                    } else {
                        couldNotFetchItemsAvailable();
                    }
                });
        }
    }
}

const getParams = (dataExplorer: DataExplorer): ListArguments => ({
    ...dataExplorerToListParams(dataExplorer),
    order: getOrder<ApiClientAuthorization>(dataExplorer),
    count: 'none',
});

const getCountParams = (): ListArguments => ({
    limit: 0,
    count: 'exact',
});

export const setItems = (listResults: ListResults<ApiClientAuthorization>) =>
    apiClientAuthorizationsActions.SET_ITEMS({
        ...listResultsToDataExplorerItemsMeta(listResults),
        items: listResults.items.map(resource => resource.uuid),
    });

const couldNotFetchLinks = () =>
    snackbarActions.OPEN_SNACKBAR({
        message: 'Could not fetch api client authorizations.',
        kind: SnackbarKind.ERROR
    });
