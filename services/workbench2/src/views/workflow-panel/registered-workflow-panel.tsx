// Copyright (C) The Arvados Authors. All rights reserved.
//
// SPDX-License-Identifier: AGPL-3.0

import React from 'react';
import { CustomStyleRulesCallback } from 'common/custom-theme';
import { Tooltip, Typography, Card, CardHeader, CardContent, IconButton } from '@mui/material';
import { WithStyles } from '@mui/styles';
import withStyles from '@mui/styles/withStyles';
import { connect, DispatchProp } from "react-redux";
import { RouteComponentProps } from 'react-router';
import { ArvadosTheme } from 'common/custom-theme';
import { RootState } from 'store/store';
import { WorkflowIcon, MoreVerticalIcon } from 'components/icon/icon';
import { WorkflowResource } from 'models/workflow';
import { ProcessOutputCollectionFiles } from 'views/process-panel/process-output-collection-files';
import { WorkflowDetailsAttributes, RegisteredWorkflowPanelDataProps, getRegisteredWorkflowPanelData } from 'views-components/details-panel/workflow-details';
import { getResource } from 'store/resources/resources';
import { openContextMenuAndSelect } from 'store/context-menu/context-menu-actions';
import { MPVContainer, MPVPanelContent, MPVPanelState } from 'components/multi-panel-view/multi-panel-view';
import { ProcessIOCard, ProcessIOCardType } from 'views/process-panel/process-io-card';
import { NotFoundView } from 'views/not-found-panel/not-found-panel';
import { WorkflowProcessesPanel } from './workflow-processes-panel';
import { resourceToMenuKind } from 'common/resource-to-menu-kind';

type CssRules =
    'root'
    | 'button'
    | 'infoCard'
    | 'propertiesCard'
    | 'filesCard'
    | 'iconHeader'
    | 'tag'
    | 'label'
    | 'value'
    | 'link'
    | 'centeredLabel'
    | 'warningLabel'
    | 'collectionName'
    | 'readOnlyIcon'
    | 'header'
    | 'title'
    | 'avatar'
    | 'content'
    | 'subHeader';

const styles: CustomStyleRulesCallback<CssRules> = (theme: ArvadosTheme) => ({
    root: {
        width: '100%',
    },
    button: {
        cursor: 'pointer'
    },
    infoCard: {
    },
    propertiesCard: {
        padding: 0,
    },
    filesCard: {
        padding: 0,
    },
    iconHeader: {
        fontSize: '1.875rem',
        color: theme.customs.colors.greyL
    },
    subHeader: {
        color: theme.customs.colors.greyD
    },
    tag: {
        marginRight: theme.spacing(0.5),
        marginBottom: theme.spacing(0.5)
    },
    label: {
        fontSize: '0.875rem',
    },
    centeredLabel: {
        fontSize: '0.875rem',
        textAlign: 'center'
    },
    warningLabel: {
        fontStyle: 'italic'
    },
    collectionName: {
        flexDirection: 'column',
    },
    value: {
        textTransform: 'none',
        fontSize: '0.875rem'
    },
    link: {
        fontSize: '0.875rem',
        color: theme.palette.primary.main,
        '&:hover': {
            cursor: 'pointer'
        }
    },
    readOnlyIcon: {
        marginLeft: theme.spacing(1),
        fontSize: 'small',
    },
    header: {
        paddingTop: theme.spacing(1),
        paddingBottom: theme.spacing(1),
    },
    title: {
        overflow: 'hidden',
        paddingTop: theme.spacing(0.5),
        color: theme.customs.colors.green700,
    },
    avatar: {
        alignSelf: 'flex-start',
        paddingTop: theme.spacing(0.5)
    },
    content: {
        padding: theme.spacing(1),
        paddingTop: theme.spacing(0.5),
        '&:last-child': {
            paddingBottom: theme.spacing(1),
        }
    }
});

type RegisteredWorkflowPanelProps = RegisteredWorkflowPanelDataProps & DispatchProp & WithStyles<CssRules>

export const RegisteredWorkflowPanel = withStyles(styles)(connect(
    (state: RootState, props: RouteComponentProps<{ id: string }>) => {
        const item = getResource<WorkflowResource>(props.match.params.id)(state.resources);
        if (item) {
            return getRegisteredWorkflowPanelData(item, state.auth);
        }
        return { item, inputParams: [], outputParams: [], workflowCollection: "", gitprops: {} };
    })(
        class extends React.Component<RegisteredWorkflowPanelProps> {
            render() {
                const { classes, item, inputParams, outputParams, workflowCollection } = this.props;
                const panelsData: MPVPanelState[] = [
                    { name: "Details" },
                    { name: "Runs" },
                    { name: "Outputs" },
                    { name: "Inputs" },
                    { name: "Definition" },
                ];
                return item
                     ? <MPVContainer className={classes.root} spacing={1} direction="column" justifyContent="flex-start" wrap="nowrap" panelStates={panelsData}>
                         <MPVPanelContent xs="auto" data-cy='registered-workflow-info-panel'>
                             <Card className={classes.infoCard}>
                                 <CardHeader
                                     className={classes.header}
                                     classes={{
                                         content: classes.title,
                                         avatar: classes.avatar,
                                     }}
                                     avatar={<WorkflowIcon className={classes.iconHeader} />}
                                     title={
                                         <Tooltip title={item.name} placement="bottom-start">
                                             <Typography noWrap variant='h6'>
                                                                         {item.name}
                                             </Typography>
                                         </Tooltip>
                                     }
                                     subheader={
                                         <div className={classes.subHeader} dangerouslySetInnerHTML={{ __html: item.description || '(no-description)' }}></div>
                                     }
                                     action={
                                         <Tooltip title="More options" disableFocusListener>
                                             <IconButton
                                                 aria-label="More options"
                                                 onClick={event => this.handleContextMenu(event)}
                                                 size="large">
                                                 <MoreVerticalIcon />
                                             </IconButton>
                                         </Tooltip>}

                                 />

                                 <CardContent className={classes.content}>
                                     <WorkflowDetailsAttributes workflow={item} />
                                 </CardContent>
                             </Card>
                         </MPVPanelContent>
                         <MPVPanelContent forwardProps xs maxHeight="100%">
                             <WorkflowProcessesPanel />
                         </MPVPanelContent>
                         <MPVPanelContent forwardProps xs data-cy="process-outputs" maxHeight="100%">
                             <ProcessIOCard
                                 label={ProcessIOCardType.OUTPUT}
                                 params={outputParams}
                                 raw={{}}
                                 forceShowParams={true}
                             />
                         </MPVPanelContent>
                         <MPVPanelContent forwardProps xs data-cy="process-inputs" maxHeight="100%">
                             <ProcessIOCard
                                 label={ProcessIOCardType.INPUT}
                                 params={inputParams}
                                 raw={{}}
                                 forceShowParams={true}
                             />
                         </MPVPanelContent>
                         <MPVPanelContent xs maxHeight="100%">
                             <Card className={classes.filesCard}>
                                 <CardHeader title="Workflow Definition" />
                                 <ProcessOutputCollectionFiles isWritable={false} currentItemUuid={workflowCollection} />
                             </Card>
                         </MPVPanelContent>
                     </MPVContainer>
 :
                       <NotFoundView
                           icon={WorkflowIcon}
                           messages={["Workflow not found"]}
                       />;
            }

            handleContextMenu = (event: React.MouseEvent<any>) => {
                const { uuid, ownerUuid, name, description,
                    kind } = this.props.item;
                const menuKind = this.props.dispatch<any>(resourceToMenuKind(uuid));
                const resource = {
                    uuid,
                    ownerUuid,
                    name,
                    description,
                    kind,
                    menuKind,
                };
                // Avoid expanding/collapsing the panel
                event.stopPropagation();
                this.props.dispatch<any>(openContextMenuAndSelect(event, resource));
            }
        }
    )
);
