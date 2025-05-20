// Copyright (C) The Arvados Authors. All rights reserved.
//
// SPDX-License-Identifier: AGPL-3.0

import React from 'react';
import { CustomStyleRulesCallback } from 'common/custom-theme';
import { CardHeader, CardContent, Typography, Grid, Link } from '@mui/material';
import { WithStyles } from '@mui/styles';
import withStyles from '@mui/styles/withStyles';
import { ArvadosTheme } from 'common/custom-theme';
import {
    ResourceIcon,
    ShowChartIcon,
} from 'components/icon/icon';
import { MPVPanelProps } from 'components/multi-panel-view/multi-panel-view';
import { connect } from 'react-redux';
import { Process } from 'store/processes/process';
import { NodeInstanceType } from 'store/process-panel/process-panel';
import { DetailsAttribute } from "components/details-attribute/details-attribute";
import { formatFileSize } from "common/formatters";
import { MountKind } from 'models/mount-types';

interface ProcessResourceCardDataProps {
    process: Process;
    nodeInfo: NodeInstanceType | null;
    usageReport: string | null;
}

type CssRules = "card" | "header" | "title" | "avatar" | "iconHeader" | "content" | "sectionH3" | "reportButton";

const styles: CustomStyleRulesCallback<CssRules> = (theme: ArvadosTheme) => ({
    card: {
        height: '100%'
    },
    header: {
        paddingBottom: "0px"
    },
    title: {
        paddingTop: theme.spacing(0.5),
    },
    avatar: {
        paddingTop: theme.spacing(0.5),
    },
    iconHeader: {
        fontSize: '1.875rem',
        color: theme.customs.colors.greyL,
    },
    content: {
        paddingTop: "0px",
        maxHeight: `calc(100% - ${theme.spacing(7.5)})`,
        overflow: "auto"
    },
    sectionH3: {
        margin: "0.5em",
        color: theme.customs.colors.greyD,
        fontSize: "0.8125rem",
        textTransform: "uppercase",
    },
    reportButton: {
    }
});

type ProcessResourceCardProps = ProcessResourceCardDataProps & WithStyles<CssRules> & MPVPanelProps;

export const ProcessResourceCard = withStyles(styles)(connect()(
    ({ classes, nodeInfo, usageReport, panelName, process, }: ProcessResourceCardProps) => {
        let diskRequest = 0;
        if (process.container?.mounts) {
            for (const mnt in process.container.mounts) {
                const mp = process.container.mounts[mnt];
                if (mp.kind === MountKind.TEMPORARY_DIRECTORY) {
                    diskRequest += mp.capacity;
                }
            }
        }

        return (
            <section className={classes.card} data-cy="process-resources-card">
                <CardHeader
                    className={classes.header}
                    classes={{
                        content: classes.title,
                        avatar: classes.avatar,
                    }}
                    avatar={<ResourceIcon className={classes.iconHeader} />}
                    title={
                        <Typography noWrap variant='h6' color='inherit'>
                                                    Resources
                        </Typography>
                    }
                    action={
                        <div>
                              {usageReport &&
                               <Link href={usageReport} className={classes.reportButton} target="_blank"><ShowChartIcon /> Resource usage report</Link>}
                        </div>
                    } />
                <CardContent className={classes.content}>
                    <Grid container>
                        <Grid item xs={4}>
                            <h3 className={classes.sectionH3}>Requested Resources</h3>
                            <Grid container>
                                <Grid item xs={12}>
                                    <DetailsAttribute label="Cores" value={process.container?.runtimeConstraints.vcpus} />
                                </Grid>
                                <Grid item xs={12}>
                                    <DetailsAttribute label="RAM*" value={formatFileSize(process.container?.runtimeConstraints.ram)} />
                                </Grid>
                                <Grid item xs={12}>
                                    <DetailsAttribute label="Disk" value={formatFileSize(diskRequest)} />
                                </Grid>

                                {process.container?.runtimeConstraints.gpu &&
                                 process.container?.runtimeConstraints.gpu.device_count > 0 ? <>
                                     <Grid item xs={12}>
                                         <DetailsAttribute label="GPU stack" value={process.container?.runtimeConstraints.gpu.stack} />
                                     </Grid>
                                <Grid item xs={12}>
                                    <DetailsAttribute label="GPU devices" value={process.container?.runtimeConstraints.gpu.device_count} />
                                </Grid>
                                <Grid item xs={12}>
                                    <DetailsAttribute label="GPU VRAM" value={process.container?.runtimeConstraints.gpu.vram} />
                                </Grid>
                                <Grid item xs={12}>
                                    <DetailsAttribute label="GPU hardware target" value={process.container?.runtimeConstraints.gpu.hardware_target} />
                                </Grid>
                                <Grid item xs={12}>
                                    <DetailsAttribute label="GPU driver version" value={process.container?.runtimeConstraints.gpu.driver_version} />
                                </Grid>
                                 </> : null}

                                {process.container?.runtimeConstraints.keep_cache_ram &&
                                 process.container?.runtimeConstraints.keep_cache_ram > 0 ?
                                                                                        <Grid item xs={12}>
                                                                                            <DetailsAttribute label="Keep cache (RAM)" value={formatFileSize(process.container?.runtimeConstraints.keep_cache_ram)} />
                                                                                        </Grid> : null}

                                {process.container?.runtimeConstraints.keep_cache_disk &&
                                 process.container?.runtimeConstraints.keep_cache_disk > 0 ?
                                                                                         <Grid item xs={12}>
                                                                                             <DetailsAttribute label="Keep cache (disk)" value={formatFileSize(process.container?.runtimeConstraints.keep_cache_disk)} />
                                                                                         </Grid> : null}

                                {process.container?.runtimeConstraints.API ? <Grid item xs={12}>
                                    <DetailsAttribute label="API access" value={process.container?.runtimeConstraints.API.toString()} />
                                </Grid> : null}

                            </Grid>
                        </Grid>


                        <Grid item xs={8}>
                            <h3 className={classes.sectionH3}>Assigned Instance Type</h3>
                            {nodeInfo === null ? <Grid item xs={8}>
                                No instance type recorded
                            </Grid>
                            :
                             <Grid container>
                                 <Grid item xs={6}>
                                     <DetailsAttribute label="Cores" value={nodeInfo.VCPUs} />
                                 </Grid>

                                 <Grid item xs={6}>
                                     <DetailsAttribute label="Provider type" value={nodeInfo.ProviderType} />
                                 </Grid>

                                 <Grid item xs={6}>
                                     <DetailsAttribute label="RAM" value={formatFileSize(nodeInfo.RAM)} />
                                 </Grid>

                                 <Grid item xs={6}>
                                     <DetailsAttribute label="Price" value={"$" + nodeInfo.Price.toString()} />
                                 </Grid>

                                 <Grid item xs={6}>
                                     <DetailsAttribute label="Disk" value={formatFileSize(nodeInfo.IncludedScratch + nodeInfo.AddedScratch)} />
                                 </Grid>

                                 <Grid item xs={6}>
                                     <DetailsAttribute label="Preemptible" value={nodeInfo.Preemptible.toString()} />
                                 </Grid>

                                 {nodeInfo.GPU && nodeInfo.GPU.DeviceCount > 0 &&
                                  <>
                                      <Grid item xs={6}>
                                          <DetailsAttribute label="GPU stack" value={nodeInfo.GPU.Stack} />
                                      </Grid>

                                 <Grid item xs={6}>
                                     <DetailsAttribute label="GPU devices" value={nodeInfo.GPU.DeviceCount} />
                                 </Grid>

                                 <Grid item xs={6}>
                                     <DetailsAttribute label="GPU VRAM" value={nodeInfo.GPU.VRAM} />
                                 </Grid>

                                 <Grid item xs={6}>
                                     <DetailsAttribute label="GPU hardware target" value={nodeInfo.GPU.HardwareTarget} />
                                 </Grid>

                                 <Grid item xs={6}>
                                     <DetailsAttribute label="GPU driver version" value={nodeInfo.GPU.DriverVersion} />
                                 </Grid>

                                  </>
                                 }
                             </Grid>}
                        </Grid>
                    </Grid>
                    <Typography>* RAM available to the program is limited to Requested RAM, not Instance RAM</Typography>
                </CardContent>
            </section >
        );
    }
));
