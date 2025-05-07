// Copyright (C) The Arvados Authors. All rights reserved.
//
// SPDX-License-Identifier: AGPL-3.0

import React, { MutableRefObject, ReactElement, ReactNode, useEffect, useRef, useState } from 'react';
import { connect } from 'react-redux';
import { RouterState } from "react-router-redux";
import { RootState } from 'store/store';
import { CustomStyleRulesCallback } from 'common/custom-theme';
import { Button, Grid, Paper, Tooltip, Tabs, Tab } from "@mui/material";
import { WithStyles } from '@mui/styles';
import withStyles from '@mui/styles/withStyles';
import { GridProps } from '@mui/material/Grid';
import { isArray, isEqual } from 'lodash';
import { DefaultView } from 'components/default-view/default-view';
import { InfoIcon } from 'components/icon/icon';
import classNames from 'classnames';

type CssRules =
    | 'exclusiveGridContainerRoot'
    | 'symmetricTabs'
    | 'gridItemRoot'
    | 'paperRoot'
    | 'button'
    | 'exclusiveContentPaper'
    | 'exclusiveContent'
    | 'tabs';

const styles: CustomStyleRulesCallback<CssRules> = theme => ({
    exclusiveGridContainerRoot: {
        marginTop: 0,
    },
    symmetricTabs: {
        "& button": {
            flexBasis: "0",
        },
    },
    gridItemRoot: {
        paddingTop: '0 !important',
        width: '100%',
    },
    paperRoot: {
        height: '100%',
        width: '100%',
        display: 'flex',
        flexDirection: 'column',
    },
    button: {
        padding: '2px 5px',
        marginRight: '5px',
    },
    exclusiveContent: {
        overflow: 'auto',
        margin: 0,
    },
    exclusiveContentPaper: {
        boxShadow: 'none',
    },
    tabs: {
        flexGrow: 1,
        flexShrink: 1,
        maxWidth: 'initial',
        borderBottom: `1px solid ${theme.palette.grey[300]}`,
    },
});

interface MPVHideablePanelDataProps {
    name: string;
    visible: boolean;
    maximized: boolean;
    illuminated: boolean;
    children: ReactNode;
    panelRef?: MutableRefObject<any>;
    paperClassName?: string;
}

interface MPVHideablePanelActionProps {
    doHidePanel: () => void;
    doMaximizePanel: () => void;
    doUnMaximizePanel: () => void;
}

type MPVHideablePanelProps = MPVHideablePanelDataProps & MPVHideablePanelActionProps;

const MPVHideablePanel = ({ doHidePanel, doMaximizePanel, doUnMaximizePanel, name, visible, maximized, illuminated, paperClassName, ...props }: MPVHideablePanelProps) =>
    visible
        ? <>
            {React.cloneElement((props.children as ReactElement), {
                doHidePanel,
                doMaximizePanel,
                doUnMaximizePanel,
                panelName: name,
                panelMaximized: maximized,
                panelIlluminated: illuminated,
                panelRef: props.panelRef,
                paperClassName,
            })}
        </>
        : null;

interface MPVPanelDataProps {
    panelName?: string;
    panelMaximized?: boolean;
    panelIlluminated?: boolean;
    panelRef?: MutableRefObject<any>;
    forwardProps?: boolean;
    maxHeight?: string;
    minHeight?: string;
    paperClassName?: string;
}

interface MPVPanelActionProps {
    doHidePanel?: () => void;
    doMaximizePanel?: () => void;
    doUnMaximizePanel?: () => void;
}

// Props received by panel implementors
export type MPVPanelProps = MPVPanelDataProps & MPVPanelActionProps;

type MPVPanelContentProps = { children: ReactElement } & MPVPanelProps & GridProps;

// Grid item compatible component for layout and MPV props passing
export const MPVPanelContent = React.memo(({ doHidePanel, doMaximizePanel, doUnMaximizePanel, panelName,
    panelMaximized, panelIlluminated, panelRef, forwardProps, maxHeight, minHeight, paperClassName,
    ...props }: MPVPanelContentProps) => {
    useEffect(() => {
        if (panelRef && panelRef.current) {
            panelRef.current.scrollIntoView({ alignToTop: true });
        }
    }, [panelRef]);

    const maxH = panelMaximized
        ? '100%'
        : maxHeight;

    return <Grid item style={{ maxHeight: maxH, minHeight, padding: '4px' }} {...props}>
        <span ref={panelRef} /> {/* Element to scroll to when the panel is selected */}
        <Paper style={{ height: '100%' }} elevation={panelIlluminated ? 8 : 0}>
            {forwardProps
                ? React.cloneElement(props.children, { doHidePanel, doMaximizePanel, doUnMaximizePanel, panelName, panelMaximized, paperClassName })
                : React.cloneElement(props.children)}
        </Paper>
    </Grid>;
}, preventRerender);

// return true to prevent re-render, false to allow re-render
function preventRerender(prevProps: MPVPanelContentProps, nextProps: MPVPanelContentProps) {
    if (!isEqual(prevProps.children, nextProps.children)) {
        return false;
    }
    if (prevProps.panelMaximized !== nextProps.panelMaximized) {
        return false;
    }
    if (prevProps.panelIlluminated !== nextProps.panelIlluminated) {
        return false;
    }
    return true;
}

export interface MPVPanelState {
    name: string;
    visible?: boolean;
}
interface MPVContainerDataProps {
    panelStates?: MPVPanelState[];
    router: RouterState;
}
type MPVContainerProps = MPVContainerDataProps & GridProps;

const mapStateToProps = (state: RootState): Pick<MPVContainerDataProps, 'router'> => ({
    router: state.router,
});

// Grid container compatible component that also handles panel toggling.
const MPVContainerComponent = ({ children, panelStates, classes, router, ...props }: MPVContainerProps & WithStyles<CssRules>) => {
    if (children === undefined || children === null || Object.keys(children).length === 0) {
        children = [];
    } else if (!isArray(children)) {
        children = [children];
    }
    const initialVisibility = (children as ReactNode[]).map((_, idx) =>
        !panelStates || // if panelStates wasn't passed, default to all visible panels
        (panelStates[idx] &&
            (panelStates[idx].visible || panelStates[idx].visible === undefined)));
    const [panelVisibility, setPanelVisibility] = useState<boolean[]>(initialVisibility);
    const [previousPanelVisibility, setPreviousPanelVisibility] = useState<boolean[]>(initialVisibility);
    const [highlightedPanel, setHighlightedPanel] = useState<number>(-1);
    const currentSelectedPanel = panelVisibility.findIndex(Boolean);
    const [selectedPanel, setSelectedPanel] = useState<number>(-1);
    const panelRef = useRef<any>(null);

    // Reset MPV to initial state when route changes
    const currentRoute = router.location ? router.location.pathname : "";
    useEffect(() => {
        setPanelVisibility(initialVisibility);
        setPreviousPanelVisibility(initialVisibility);
        setHighlightedPanel(-1);
        setSelectedPanel(-1);
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [currentRoute]); // Omit initialVisibility to avoid infinite loops

    let panels: JSX.Element[] = [];
    let buttons: JSX.Element[] = [];
    let tabs: JSX.Element[] = [];
    let buttonBar: JSX.Element = <></>;

    if (isArray(children)) {
        const showFn = (idx: number) => () => {
            setPreviousPanelVisibility(initialVisibility);
            // Hide all other panels
            setPanelVisibility([
                ...(new Array(idx).fill(false)),
                true,
                ...(new Array(panelVisibility.length-(idx+1)).fill(false)),
            ]);
            setSelectedPanel(idx);
        };
        const hideFn = (idx: number) => () => {
            setPreviousPanelVisibility(initialVisibility);
            setPanelVisibility([
                ...panelVisibility.slice(0, idx),
                false,
                ...panelVisibility.slice(idx+1)
            ])
        };
        const maximizeFn = (idx: number) => () => {
            setPreviousPanelVisibility(panelVisibility);
            // Maximize X == hide all but X
            setPanelVisibility([
                ...panelVisibility.slice(0, idx).map(() => false),
                true,
                ...panelVisibility.slice(idx+1).map(() => false),
            ]);
        };
        const unMaximizeFn = (idx: number) => () => {
            setPanelVisibility(previousPanelVisibility);
            setSelectedPanel(idx);
        }
        for (let idx = 0; idx < children.length; idx++) {
            const panelName = panelStates === undefined
                ? `Panel ${idx + 1}`
                : (panelStates[idx] && panelStates[idx].name) || `Panel ${idx + 1}`;
            const btnVariant = panelVisibility[idx]
                ? "contained"
                : "outlined";
            const btnTooltip = panelVisibility[idx]
                ? ``
                : `Open ${panelName} panel`;
            const panelIsMaximized = panelVisibility[idx] &&
                panelVisibility.filter(e => e).length === 1;

            buttons = [
                ...buttons,
                <Tooltip title={btnTooltip} disableFocusListener>
                    <Button variant={btnVariant} size="small" color="primary"
                        className={classNames(classes.button)}
                        onMouseEnter={() => {
                            setHighlightedPanel(idx);
                        }}
                        onMouseLeave={() => {
                            setHighlightedPanel(-1);
                        }}
                        onClick={showFn(idx)}>
                        {panelName}
                    </Button>
                </Tooltip>
            ];

            tabs = [
                ...tabs,
                <>{panelName}</>
            ];

            const aPanel =
                <MPVHideablePanel
                    key={idx}
                    visible={panelVisibility[idx]}
                    name={panelName}
                    paperClassName={classes.exclusiveContentPaper}
                    panelRef={(idx === selectedPanel) ? panelRef : undefined}
                    maximized={panelIsMaximized} illuminated={idx === highlightedPanel}
                    doHidePanel={hideFn(idx)} doMaximizePanel={maximizeFn(idx)} doUnMaximizePanel={panelIsMaximized ? unMaximizeFn(idx) : () => null}>
                    {children[idx]}
                </MPVHideablePanel>;
            panels = [...panels, aPanel];
        };

        buttonBar = (
            <Tabs className={classes.symmetricTabs} value={currentSelectedPanel} onChange={(e, val) => showFn(val)()} data-cy={"mpv-tabs"}>
                {tabs.map((tgl, idx) => <Tab className={classes.tabs} key={idx} label={tgl} />)}
            </Tabs>);
    };

    const content = <Grid container direction="column" item {...props} xs className={classes.exclusiveContent}
        onScroll={() => setSelectedPanel(-1)}>
        {panelVisibility.includes(true)
            ? panels
            : <Grid container item alignItems='center' justifyContent='center'>
                <DefaultView messages={["All panels are hidden.", "Click on the buttons above to show them."]} icon={InfoIcon} />
            </Grid>}
    </Grid>;

        return (
            <Grid container {...props} className={classNames(classes.exclusiveGridContainerRoot, props.className)}>
                <Grid item {...props} className={classes.gridItemRoot}>
                    <Paper className={classes.paperRoot}>
                        {buttonBar}
                        {content}
                    </Paper>
                </Grid>
            </Grid>);
};

export const MPVContainer = connect(mapStateToProps)(withStyles(styles)(MPVContainerComponent));
