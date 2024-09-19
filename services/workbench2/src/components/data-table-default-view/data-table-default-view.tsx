// Copyright (C) The Arvados Authors. All rights reserved.
//
// SPDX-License-Identifier: AGPL-3.0

import React from 'react';
import { CustomStyleRulesCallback } from 'common/custom-theme';
import { WithStyles } from '@mui/styles';
import withStyles from '@mui/styles/withStyles';
import { DefaultViewDataProps, DefaultView } from 'components/default-view/default-view';
import { ArvadosTheme } from 'common/custom-theme';
import { DetailsIcon } from 'components/icon/icon';

type CssRules = 'classRoot';

const styles: CustomStyleRulesCallback<CssRules> = (theme: ArvadosTheme) => ({
    classRoot: {
        marginTop: theme.spacing(4),
        marginBottom: theme.spacing(4),
    },
});
type DataTableDefaultViewDataProps = Partial<Pick<DefaultViewDataProps, 'icon' | 'messages' | 'filtersApplied'>>;
type DataTableDefaultViewProps = DataTableDefaultViewDataProps & WithStyles<CssRules>;

export const DataTableDefaultView = withStyles(styles)(
    ({ classes, ...props }: DataTableDefaultViewProps) => {
        const icon = props.icon || DetailsIcon;
        const filterWarning: string[] = props.filtersApplied ? ['Filters are applied to the data.'] : [];
        const messages = filterWarning.concat(props.messages || ['No items found']);
        return <DefaultView {...classes} {...{ icon, messages }} />;
    });
