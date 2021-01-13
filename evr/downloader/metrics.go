// Copyright 2015 The evrynet-node Authors
// This file is part of the evrynet-node library.
//
// The evrynet-node library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The evrynet-node library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the evrynet-node library. If not, see <http://www.gnu.org/licenses/>.

// Contains the metrics collected by the downloader.

package downloader

import (
	"github.com/Evrynetlabs/evrynet-node/metrics"
)

var (
	headerInMeter      = metrics.NewRegisteredMeter("evr/downloader/headers/in", nil)
	headerReqTimer     = metrics.NewRegisteredTimer("evr/downloader/headers/req", nil)
	headerDropMeter    = metrics.NewRegisteredMeter("evr/downloader/headers/drop", nil)
	headerTimeoutMeter = metrics.NewRegisteredMeter("evr/downloader/headers/timeout", nil)

	bodyInMeter          = metrics.NewRegisteredMeter("evr/downloader/bodies/in", nil)
	bodyReqTimer         = metrics.NewRegisteredTimer("evr/downloader/bodies/req", nil)
	bodyDropMeter        = metrics.NewRegisteredMeter("evr/downloader/bodies/drop", nil)
	bodyTimeoutMeter     = metrics.NewRegisteredMeter("evr/downloader/bodies/timeout", nil)

	receiptInMeter      = metrics.NewRegisteredMeter("evr/downloader/receipts/in", nil)
	receiptReqTimer     = metrics.NewRegisteredTimer("evr/downloader/receipts/req", nil)
	receiptDropMeter    = metrics.NewRegisteredMeter("evr/downloader/receipts/drop", nil)
	receiptTimeoutMeter = metrics.NewRegisteredMeter("evr/downloader/receipts/timeout", nil)

	stateInMeter   = metrics.NewRegisteredMeter("evr/downloader/states/in", nil)
	stateDropMeter = metrics.NewRegisteredMeter("evr/downloader/states/drop", nil)

	evilBodyInMeter          = metrics.NewRegisteredMeter("evr/downloader/evilBodies/in", nil)
	evilBodyReqTimer         = metrics.NewRegisteredTimer("evr/downloader/evilBodies/req", nil)
	evilBodyDropMeter        = metrics.NewRegisteredMeter("evr/downloader/evilBodies/drop", nil)
	evilBodyTimeoutMeter     = metrics.NewRegisteredMeter("evr/downloader/evilBodies/timeout", nil)
)
