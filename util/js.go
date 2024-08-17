package util

const CommonJS = `
(function($){
	var AutoCompleter = function(element, settings){
		var elem = $(element);
		var obj = this;
		obj.settings = $.extend({
			url: "/autocomplete", // url used for the autocomplete
			resultKey: "results", // name of the key in the JSON result
			method: "GET", // method to do the AJAX request
			delay: 100, // delay in ms between invocation of the autocomplete
			minChars: 1, // minimum number of characters to have in order to trigger the autocomplete
			maxQueriesToCache: 5, // hide five queries
			autocompleterClass: "autocompleter", //class of the autocompleter
			autocompleterResultClass: "autocompleterResult", 
			queryCallback: null, // query to do the callback
			selectRowCallback: null, // callback triggered when a row is selected
			confirmSuggestionCallback: null, // callback after a suggestion is confirmed
			createResultRowCallback: null, // callback to create a row to add to the autocomplete popup
			footerLink: null, // if	!= null, the link will be added to the bottom of the autocomplete list
			footerLinkCallback: null, // if != null, the function will be called clicking on the footerLink
		    closeAfterSelect: true // if true, close the autocomplete box after clicking on an item
		}, settings || {});

		obj.cacheData = {};
		obj.originalValue = null;
		obj.reqId = 0;
		obj.popup = elem.next("." + obj.settings.autocompleterClass);
		if (obj.popup.length > 0)
			obj.popup.remove();
		obj.popup = $("<div class='" + obj.settings.autocompleterClass + "' style='display: none;'/>");
		elem.after(obj.popup);
		elem.attr("autocomplete", "off");

		// Closes the popup and rolls back the changes
		obj.killPopup = function() {
			if (obj.isPopupVisible()) {
				obj.originalValue = null;
				obj.popup.hide();
			}
		};

		obj.isPopupVisible = function() {
			return obj.popup.is(":visible");
		};

		obj.processKey = function(e) {
			var isPopupVisible = obj.isPopupVisible();
			switch (e.which) {
				case 40: // down
					var next = obj.nextSuggestion();
					obj.selectRow(next);
					break;
				case 38: // up
					var previous = obj.previousSuggestion();
					obj.selectRow(previous);
					break;
				case 39: // right
				case 37: // left arrow
					break;
				case 13: // enter
					obj.confirmSuggestion();
					break;
				case 27: // escape
					elem.blur();
					break;
				default:
					obj.delayedAutoComplete();
					break;
			};
		};

		obj.getSelectedRow = function() {
			return obj.popup.find(".current");
		};

		obj.previousSuggestion = function(){
			var selectedRow = obj.getSelectedRow();
			if (selectedRow.length == 1){
                var results = obj.popup.find("." + obj.settings.autocompleterResultClass);
				var index = results.index(selectedRow);
                var result = null;
				if(index == 0)
					result = obj.footerLink;
				else if(index > 0)
					result = $(results[index - 1]);
				else
					result = results.last();
				return result;
			} else {
				return obj.footerLink;
			}
		};

		obj.nextSuggestion = function() {
			var selectedRow = obj.getSelectedRow();
			var results = obj.popup.find("." + obj.settings.autocompleterResultClass);
			if (selectedRow.length == 1) {
				var result = null;
				var index = results.index(selectedRow);
				if (index + 1 < results.length)
					result = $(results[index + 1]);
				else
					result = obj.footerLink;
				return result;
			} else {
				if (results.length > 0)
					return results.first();
				else
					return obj.footerLink;
			}
		};

		obj.rollback = function() {
			if (obj.originalValue != null)
				elem.val(obj.originalValue);
		};

		obj.delayedAutoComplete = function() {
			if (obj.settings.delay) {
				obj.reqId++;
				setTimeout(function () { obj.autoComplete(obj.reqId); }, obj.settings.delay);
			}
		};

		obj.confirmSuggestion = function() {
			var selectedRow = obj.getSelectedRow();
            if (selectedRow.length == 1) {
                if (selectedRow.attr("class").indexOf("footerLink") >= 0 && obj.settings.footerLinkCallback) {
                    obj.settings.footerLinkCallback.call();
                } else if (obj.settings.confirmSuggestionCallback) {
                    obj.settings.confirmSuggestionCallback.call(obj, selectedRow);
                }
            }
			
			if (obj.settings.closeAfterSelect)
				obj.killPopup();
		};

		obj.autoComplete = function(reqId) {

			// Make sure we only execute the autocomplete for the *last* input action
			if (obj.reqId != reqId)
				return;

			var criterion = elem.val();
			obj.originalValue = criterion;

			// Not enough characters in the input, move along
			if (obj.settings.minChars && criterion.length < obj.settings.minChars){
				obj.handleQueryResults(null, reqId);
				obj.popup.css('height', '');
				return;
			}

			// cache
			var resultsHash = obj.cache(obj.settings.url + ":" + criterion);
			if (resultsHash){
				obj.handleQueryResults(resultsHash, reqId);
				return;
			}

			// no cache
			obj.settings.queryCallback.call(obj, function (results) {
				obj.cache(obj.settings.url + ":" + criterion, results);
				obj.handleQueryResults(results, reqId);
			});
		};

		obj.cache = function(criterion, resultsHash) {

			// we want to retrieve the results
			if (resultsHash == null) {
				var resultsData = obj.cacheData[criterion];
				if (resultsData) {
					// update last access date
					resultsData.lastAccess = new Date().getTime();
					return resultsData.results;
				}
				else
					return null;
			}

			// we want to store them
			else {
				if (obj.cacheData.length >= obj.settings.maxQueriesToCache) {
					// oups, we're exceeding our allowance, discard the oldest query
					var oldestTime = new Date().getTime();
					var oldestCrit = null;
					for (var key in cache) {
						var accessTime = obj.cacheData[key].latestAccess;
						if (accessTime < oldestTime) {
							oldestCrit = key;
							oldestTime = accessTime;
						}
					}
					delete obj.cacheData[oldestCrit];
				}

				// actually store in cache
				obj.cacheData[criterion] = {
					results: resultsHash,
					lastAccess: new Date().getTime()
				};
			}
		};

		obj.handleQueryResults = function(resultsHash, reqId) {

			// Make sure we only execute the autocomplete for the *last* input action
			if (this.reqId != reqId)
				return;

			var results = null;
			var key = obj.settings.resultKey;

			obj.popup.empty();

			if (resultsHash == null || resultsHash[key] == null)
				return;

			results = resultsHash[key];
			var table = $("<ul />");
			for(var i in results) {
				var val = results[i];
				var row = obj.settings.createResultRowCallback.call(obj, i, val);
				if (row)
					table.append(row);
			}
            obj.popup.append(table);

			//append the footer link
			if (obj.settings.footerLink != null) {
				var resultListLink = $("<a class='footerLink' data-value='AllResults'>" + obj.settings.footerLink + "</a>");
				obj.footerLink = resultListLink;
				obj.popup.append(resultListLink);
                if (obj.settings.footerLinkCallback != null) {
                    resultListLink.mousedown(function() {
                        return obj.settings.footerLinkCallback.call();
                    });
                }
			}

			//click
			obj.popup.find("." + obj.settings.autocompleterResultClass).mousedown(function(e) {
				obj.selectRow($(this));
				obj.confirmSuggestion();
				if (!obj.settings.closeAfterSelect)
					e.preventDefault();
			});
		};

		obj.showPopup = function() {
			if ($("#keyboard:visible, #arabicKeyboard:visible, #russianKeyboard:visible, #keyboardList:visible").length > 0) {
				return;
			}
			obj.popup.show();
		};

		obj.selectRow = function(row) {
			obj.popup.find(".current").removeClass("current");
			if (row != null){
                row.addClass("current");
                if (row.attr("class").indexOf("footerLink") >= 0)
                    elem.val(this.originalValue);
                else
                    elem.val(row.text());
			}else{
				obj.rollback();
			}

			if (this.settings.selectRowCallback)
				this.settings.selectRowCallback.call();
		};

		elem.off("change.autocomplete");
		elem.on("change.autocomplete", function(e){
			obj.delayedAutoComplete(e);
		});
		elem.off("input.autocomplete");
		elem.on("input.autocomplete", function(e){
			setTimeout(function () { elem.trigger("change"); }, 100);
		});
		elem.off("keyup.autocomplete");
		elem.on("keyup.autocomplete", function(e){
			obj.processKey(e);
		});
		elem.off("blur.autocomplete");
		elem.on("blur.autocomplete", function(){
			obj.killPopup();
		});
		elem.off("focus.autocomplete");
		elem.on("focus.autocomplete", function(){
			obj.showPopup();
		});
	};

	$.fn.autoCompleter = function(settings) {
		return this.each(function() {
			var element = $(this);
			var autoCompleter = new AutoCompleter(this, settings);
			element.data('autoCompleter', autoCompleter);
		});
	};
})(jQuery);/*!
 * # Semantic UI 2.2.11 - Dropdown
 * http://github.com/semantic-org/semantic-ui/
 *
 *
 * Released under the MIT license
 * http://opensource.org/licenses/MIT
 *
 */
!function(e,t,n,i){"use strict";t=void 0!==t&&t.Math==Math?t:"undefined"!=typeof self&&self.Math==Math?self:Function("return this")(),e.fn.dropdown=function(i){var o,a=e(this),s=e(n),r=a.selector||"",l="ontouchstart"in n.documentElement,c=(new Date).getTime(),u=[],d=arguments[0],v="string"==typeof d,f=[].slice.call(arguments,1);return a.each(function(m){var h,g,p,b,w,x,C,S,y=e.isPlainObject(i)?e.extend(!0,{},e.fn.dropdown.settings,i):e.extend({},e.fn.dropdown.settings),A=y.className,T=y.message,k=y.fields,L=y.keys,I=y.metadata,D=y.namespace,q=y.regExp,R=y.selector,O=y.error,V=y.templates,E="."+D,F="module-"+D,M=e(this),z=e(y.context),P=M.find(R.text),H=M.find(R.search),j=M.find(R.sizer),N=M.find(R.input),U=M.find(R.icon),K=M.prev().find(R.text).length>0?M.prev().find(R.text):M.prev(),W=M.children(R.menu),B=W.find(R.item),$=!1,Q=!1,X=!1,Y=this,G=M.data(F);S={initialize:function(){S.debug("Initializing dropdown",y),S.is.alreadySetup()?S.setup.reference():(S.setup.layout(),S.refreshData(),S.save.defaults(),S.restore.selected(),S.create.id(),S.bind.events(),S.observeChanges(),S.instantiate())},instantiate:function(){S.verbose("Storing instance of dropdown",S),G=S,M.data(F,S)},destroy:function(){S.verbose("Destroying previous dropdown",M),S.remove.tabbable(),M.off(E).removeData(F),W.off(E),s.off(b),S.disconnect.menuObserver(),S.disconnect.selectObserver()},observeChanges:function(){"MutationObserver"in t&&(x=new MutationObserver(S.event.select.mutation),C=new MutationObserver(S.event.menu.mutation),S.debug("Setting up mutation observer",x,C),S.observe.select(),S.observe.menu())},disconnect:{menuObserver:function(){C&&C.disconnect()},selectObserver:function(){x&&x.disconnect()}},observe:{select:function(){S.has.input()&&x.observe(N[0],{childList:!0,subtree:!0})},menu:function(){S.has.menu()&&C.observe(W[0],{childList:!0,subtree:!0})}},create:{id:function(){w=(Math.random().toString(16)+"000000000").substr(2,8),b="."+w,S.verbose("Creating unique id for element",w)},userChoice:function(t){var n,i,o;return!!(t=t||S.get.userValues())&&(t=Array.isArray(t)?t:[t],e.each(t,function(t,a){!1===S.get.item(a)&&(o=y.templates.addition(S.add.variables(T.addResult,a)),i=e("<div />").html(o).attr("data-"+I.value,a).attr("data-"+I.text,a).addClass(A.addition).addClass(A.item),y.hideAdditions&&i.addClass(A.hidden),n=void 0===n?i:n.add(i),S.verbose("Creating user choices for value",a,i))}),n)},userLabels:function(t){var n=S.get.userValues();n&&(S.debug("Adding user labels",n),e.each(n,function(e,t){S.verbose("Adding custom user value"),S.add.label(t,t)}))},menu:function(){W=e("<div />").addClass(A.menu).appendTo(M)},sizer:function(){j=e("<span />").addClass(A.sizer).insertAfter(H)}},search:function(e){e=void 0!==e?e:S.get.query(),S.verbose("Searching for query",e),S.has.minCharacters(e)?S.filter(e):S.hide()},select:{firstUnfiltered:function(){S.verbose("Selecting first non-filtered element"),S.remove.selectedItem(),B.not(R.unselectable).not(R.addition+R.hidden).eq(0).addClass(A.selected)},nextAvailable:function(e){e=e.eq(0);var t=e.nextAll(R.item).not(R.unselectable).eq(0),n=e.prevAll(R.item).not(R.unselectable).eq(0);t.length>0?(S.verbose("Moving selection to",t),t.addClass(A.selected)):(S.verbose("Moving selection to",n),n.addClass(A.selected))}},setup:{api:function(){var e={debug:y.debug,urlData:{value:S.get.value(),query:S.get.query()},on:!1};S.verbose("First request, initializing API"),M.api(e)},layout:function(){M.is("select")&&(S.setup.select(),S.setup.returnedObject()),S.has.menu()||S.create.menu(),S.is.search()&&!S.has.search()&&(S.verbose("Adding search input"),H=e("<input />").addClass(A.search).prop("autocomplete","off").insertBefore(P)),S.is.multiple()&&S.is.searchSelection()&&!S.has.sizer()&&S.create.sizer(),y.allowTab&&S.set.tabbable()},select:function(){var t=S.get.selectValues();S.debug("Dropdown initialized on a select",t),M.is("select")&&(N=M),N.parent(R.dropdown).length>0?(S.debug("UI dropdown already exists. Creating dropdown menu only"),M=N.closest(R.dropdown),S.has.menu()||S.create.menu(),W=M.children(R.menu),S.setup.menu(t)):(S.debug("Creating entire dropdown from select"),M=e("<div />").attr("class",N.attr("class")).addClass(A.selection).addClass(A.dropdown).html(V.dropdown(t)).insertBefore(N),N.hasClass(A.multiple)&&!1===N.prop("multiple")&&(S.error(O.missingMultiple),N.prop("multiple",!0)),N.is("[multiple]")&&S.set.multiple(),N.prop("disabled")&&(S.debug("Disabling dropdown"),M.addClass(A.disabled)),N.removeAttr("class").detach().prependTo(M)),S.refresh()},menu:function(e){W.html(V.menu(e,k)),B=W.find(R.item)},reference:function(){S.debug("Dropdown behavior was called on select, replacing with closest dropdown"),M=M.parent(R.dropdown),S.refresh(),S.setup.returnedObject(),v&&(G=S,S.invoke(d))},returnedObject:function(){var e=a.slice(0,m),t=a.slice(m+1);a=e.add(M).add(t)}},refresh:function(){S.refreshSelectors(),S.refreshData()},refreshItems:function(){B=W.find(R.item)},refreshSelectors:function(){S.verbose("Refreshing selector cache"),P=M.find(R.text),H=M.find(R.search),N=M.find(R.input),U=M.find(R.icon),K=M.prev().find(R.text).length>0?M.prev().find(R.text):M.prev(),W=M.children(R.menu),B=W.find(R.item)},refreshData:function(){S.verbose("Refreshing cached metadata"),B.removeData(I.text).removeData(I.value)},clearData:function(){S.verbose("Clearing metadata"),B.removeData(I.text).removeData(I.value),M.removeData(I.defaultText).removeData(I.defaultValue).removeData(I.placeholderText)},toggle:function(){S.verbose("Toggling menu visibility"),S.is.active()?S.hide():S.show()},show:function(t){if(t=e.isFunction(t)?t:function(){},!S.can.show()&&S.is.remote()&&(S.debug("No API results retrieved, searching before show"),S.queryRemote(S.get.query(),S.show)),S.can.show()&&!S.is.active()){if(S.debug("Showing dropdown"),!S.has.message()||S.has.maxSelections()||S.has.allResultsFiltered()||S.remove.message(),S.is.allFiltered())return!0;!1!==y.onShow.call(Y)&&S.animate.show(function(){S.can.click()&&S.bind.intent(),S.has.menuSearch()&&S.focusSearch(),S.set.visible(),t.call(Y)})}},hide:function(t){t=e.isFunction(t)?t:function(){},S.is.active()&&(S.debug("Hiding dropdown"),!1!==y.onHide.call(Y)&&S.animate.hide(function(){S.remove.visible(),t.call(Y)}))},hideOthers:function(){S.verbose("Finding other dropdowns to hide"),a.not(M).has(R.menu+"."+A.visible).dropdown("hide")},hideMenu:function(){S.verbose("Hiding menu  instantaneously"),S.remove.active(),S.remove.visible(),W.transition("hide")},hideSubMenus:function(){var e=W.children(R.item).find(R.menu);S.verbose("Hiding sub menus",e),e.transition("hide")},bind:{events:function(){l&&S.bind.touchEvents(),S.bind.keyboardEvents(),S.bind.inputEvents(),S.bind.mouseEvents()},touchEvents:function(){S.debug("Touch device detected binding additional touch events"),S.is.searchSelection()||S.is.single()&&M.on("touchstart"+E,S.event.test.toggle),W.on("touchstart"+E,R.item,S.event.item.mouseenter)},keyboardEvents:function(){S.verbose("Binding keyboard events"),M.on("keydown"+E,S.event.keydown),S.has.search()&&M.on(S.get.inputEvent()+E,R.search,S.event.input),S.is.multiple()&&s.on("keydown"+b,S.event.document.keydown)},inputEvents:function(){S.verbose("Binding input change events"),M.on("change"+E,R.input,S.event.change)},mouseEvents:function(){S.verbose("Binding mouse events"),S.is.multiple()&&M.on("click"+E,R.label,S.event.label.click).on("click"+E,R.remove,S.event.remove.click),S.is.searchSelection()?(M.on("mousedown"+E,S.event.mousedown).on("mouseup"+E,S.event.mouseup).on("mousedown"+E,R.menu,S.event.menu.mousedown).on("mouseup"+E,R.menu,S.event.menu.mouseup).on("click"+E,R.icon,S.event.icon.click).on("focus"+E,R.search,S.event.search.focus).on("click"+E,R.search,S.event.search.focus).on("blur"+E,R.search,S.event.search.blur).on("click"+E,R.text,S.event.text.focus),S.is.multiple()&&M.on("click"+E,S.event.click)):("click"==y.on?M.on("click"+E,R.icon,S.event.icon.click).on("click"+E,S.event.test.toggle):"hover"==y.on?M.on("mouseenter"+E,S.delay.show).on("mouseleave"+E,S.delay.hide):M.on(y.on+E,S.toggle),M.on("mousedown"+E,S.event.mousedown).on("mouseup"+E,S.event.mouseup).on("focus"+E,S.event.focus),S.has.menuSearch()?M.on("blur"+E,R.search,S.event.search.blur):M.on("blur"+E,S.event.blur)),W.on("mouseenter"+E,R.item,S.event.item.mouseenter).on("mouseleave"+E,R.item,S.event.item.mouseleave).on("click"+E,R.item,S.event.item.click)},intent:function(){S.verbose("Binding hide intent event to document"),l&&s.on("touchstart"+b,S.event.test.touch).on("touchmove"+b,S.event.test.touch),s.on("click"+b,S.event.test.hide)}},unbind:{intent:function(){S.verbose("Removing hide intent event from document"),l&&s.off("touchstart"+b).off("touchmove"+b),s.off("click"+b)}},filter:function(e){var t=void 0!==e?e:S.get.query(),n=function(){S.is.multiple()&&S.filterActive(),(e||!e&&0==S.get.activeItem().length)&&S.select.firstUnfiltered(),S.has.allResultsFiltered()?y.onNoResults.call(Y,t)?y.allowAdditions?y.hideAdditions&&(S.verbose("User addition with no menu, setting empty style"),S.set.empty(),S.hideMenu()):(S.verbose("All items filtered, showing message",t),S.add.message(T.noResults)):(S.verbose("All items filtered, hiding dropdown",t),S.hideMenu()):(S.remove.empty(),S.remove.message()),y.allowAdditions&&S.add.userSuggestion(e),S.is.searchSelection()&&S.can.show()&&S.is.focusedOnSearch()&&S.show()};y.useLabels&&S.has.maxSelections()||(y.apiSettings?S.can.useAPI()?S.queryRemote(t,function(){y.filterRemoteData&&S.filterItems(t),n()}):S.error(O.noAPI):(S.filterItems(t),n()))},queryRemote:function(t,n){var i={errorDuration:!1,cache:"local",throttle:y.throttle,urlData:{query:t},onError:function(){S.add.message(T.serverError),n()},onFailure:function(){S.add.message(T.serverError),n()},onSuccess:function(e){S.remove.message(),S.setup.menu({values:e[k.remoteValues]}),n()}};M.api("get request")||S.setup.api(),i=e.extend(!0,{},i,y.apiSettings),M.api("setting",i).api("query")},filterItems:function(t){var n=void 0!==t?t:S.get.query(),i=null,o=S.escape.string(n),a=new RegExp("^"+o,"igm");S.has.query()&&(i=[],S.verbose("Searching for matching values",n),B.each(function(){var t,o,s=e(this);if("both"==y.match||"text"==y.match){if(t=String(S.get.choiceText(s,!1)),-1!==t.search(a))return i.push(this),!0;if("exact"===y.fullTextSearch&&S.exactSearch(n,t))return i.push(this),!0;if(!0===y.fullTextSearch&&S.fuzzySearch(n,t))return i.push(this),!0}if("both"==y.match||"value"==y.match){if(o=String(S.get.choiceValue(s,t)),-1!==o.search(a))return i.push(this),!0;if("exact"===y.fullTextSearch&&S.exactSearch(n,o))return i.push(this),!0;if(!0===y.fullTextSearch&&S.fuzzySearch(n,o))return i.push(this),!0}})),S.debug("Showing only matched items",n),S.remove.filteredItem(),i&&B.not(i).addClass(A.filtered)},fuzzySearch:function(e,t){var n=t.length,i=e.length;if(e=e.toLowerCase(),t=t.toLowerCase(),i>n)return!1;if(i===n)return e===t;e:for(var o=0,a=0;o<i;o++){for(var s=e.charCodeAt(o);a<n;)if(t.charCodeAt(a++)===s)continue e;return!1}return!0},exactSearch:function(e,t){return e=e.toLowerCase(),t=t.toLowerCase(),t.indexOf(e)>-1},filterActive:function(){y.useLabels&&B.filter("."+A.active).addClass(A.filtered)},focusSearch:function(e){S.has.search()&&!S.is.focusedOnSearch()&&(e?(M.off("focus"+E,R.search),H.focus(),M.on("focus"+E,R.search,S.event.search.focus)):H.focus())},forceSelection:function(){var e=B.not(A.filtered).filter("."+A.selected).eq(0),t=B.not(A.filtered).filter("."+A.active).eq(0),n=e.length>0?e:t;if(n.length>0&&!S.is.multiple())return S.debug("Forcing partial selection to selected item",n),void S.event.item.click.call(n,{},!0);y.allowAdditions?(S.set.selected(S.get.query()),S.remove.searchTerm()):S.remove.searchTerm()},event:{change:function(){X||(S.debug("Input changed, updating selection"),S.set.selected())},focus:function(){y.showOnFocus&&!$&&S.is.hidden()&&!g&&S.show()},blur:function(e){g=n.activeElement===this,$||g||(S.remove.activeLabel(),S.hide())},mousedown:function(){S.is.searchSelection()?p=!0:$=!0},mouseup:function(){S.is.searchSelection()?p=!1:$=!1},click:function(t){e(t.target).is(M)&&(S.is.focusedOnSearch()?S.show():S.focusSearch())},search:{focus:function(){$=!0,S.is.multiple()&&S.remove.activeLabel(),y.showOnFocus&&S.search()},blur:function(e){g=n.activeElement===this,S.is.searchSelection()&&!p&&(Q||g||(y.forceSelection&&S.forceSelection(),S.hide())),p=!1}},icon:{click:function(e){S.toggle()}},text:{focus:function(e){$=!0,S.focusSearch()}},input:function(e){(S.is.multiple()||S.is.searchSelection())&&S.set.filtered(),clearTimeout(S.timer),S.timer=setTimeout(S.search,y.delay.search)},label:{click:function(t){var n=e(this),i=M.find(R.label),o=i.filter("."+A.active),a=n.nextAll("."+A.active),s=n.prevAll("."+A.active),r=a.length>0?n.nextUntil(a).add(o).add(n):n.prevUntil(s).add(o).add(n);t.shiftKey?(o.removeClass(A.active),r.addClass(A.active)):t.ctrlKey?n.toggleClass(A.active):(o.removeClass(A.active),n.addClass(A.active)),y.onLabelSelect.apply(this,i.filter("."+A.active))}},remove:{click:function(){var t=e(this).parent();t.hasClass(A.active)?S.remove.activeLabels():S.remove.activeLabels(t)}},test:{toggle:function(e){var t=S.is.multiple()?S.show:S.toggle;S.is.bubbledLabelClick(e)||S.is.bubbledIconClick(e)||S.determine.eventOnElement(e,t)&&e.preventDefault()},touch:function(e){S.determine.eventOnElement(e,function(){"touchstart"==e.type?S.timer=setTimeout(function(){S.hide()},y.delay.touch):"touchmove"==e.type&&clearTimeout(S.timer)}),e.stopPropagation()},hide:function(e){S.determine.eventInModule(e,S.hide)}},select:{mutation:function(e){S.debug("<select> modified, recreating menu"),S.setup.select()}},menu:{mutation:function(t){var n=t[0],i=e(n.addedNodes?n.addedNodes[0]:!1),o=e(n.removedNodes?n.removedNodes[0]:!1),a=i.add(o),s=a.is(R.addition)||a.closest(R.addition).length>0,r=a.is(R.message)||a.closest(R.message).length>0;s||r?(S.debug("Updating item selector cache"),S.refreshItems()):(S.debug("Menu modified, updating selector cache"),S.refresh())},mousedown:function(){Q=!0},mouseup:function(){Q=!1}},item:{mouseenter:function(t){var n=e(t.target),i=e(this),o=i.children(R.menu),a=i.siblings(R.item).children(R.menu),s=o.length>0;!(o.find(n).length>0)&&s&&(clearTimeout(S.itemTimer),S.itemTimer=setTimeout(function(){S.verbose("Showing sub-menu",o),e.each(a,function(){S.animate.hide(!1,e(this))}),S.animate.show(!1,o)},y.delay.show),t.preventDefault())},mouseleave:function(t){var n=e(this).children(R.menu);n.length>0&&(clearTimeout(S.itemTimer),S.itemTimer=setTimeout(function(){S.verbose("Hiding sub-menu",n),S.animate.hide(!1,n)},y.delay.hide))},click:function(t,i){var o=e(this),a=e(t?t.target:""),s=o.find(R.menu),r=S.get.choiceText(o),l=S.get.choiceValue(o,r),c=s.length>0,u=s.find(a).length>0;S.has.menuSearch()&&e(n.activeElement).blur(),u||c&&!y.allowCategorySelection||(S.is.searchSelection()&&(y.allowAdditions&&S.remove.userAddition(),S.remove.searchTerm(),S.is.focusedOnSearch()||1==i||S.focusSearch(!0)),y.useLabels||(S.remove.filteredItem(),S.set.scrollPosition(o)),S.determine.selectAction.call(this,r,l))}},document:{keydown:function(e){var t=e.which;if(S.is.inObject(t,L)){var n=M.find(R.label),i=n.filter("."+A.active),o=(i.data(I.value),n.index(i)),a=n.length,s=i.length>0,r=i.length>1,l=0===o,c=o+1==a,u=S.is.searchSelection(),d=S.is.focusedOnSearch(),v=S.is.focused(),f=d&&0===S.get.caretPosition();if(u&&!s&&!d)return;t==L.leftArrow?!v&&!f||s?s&&(e.shiftKey?S.verbose("Adding previous label to selection"):(S.verbose("Selecting previous label"),n.removeClass(A.active)),l&&!r?i.addClass(A.active):i.prev(R.siblingLabel).addClass(A.active).end(),e.preventDefault()):(S.verbose("Selecting previous label"),n.last().addClass(A.active)):t==L.rightArrow?(v&&!s&&n.first().addClass(A.active),s&&(e.shiftKey?S.verbose("Adding next label to selection"):(S.verbose("Selecting next label"),n.removeClass(A.active)),c?u?d?n.removeClass(A.active):S.focusSearch():r?i.next(R.siblingLabel).addClass(A.active):i.addClass(A.active):i.next(R.siblingLabel).addClass(A.active),e.preventDefault())):t==L.deleteKey||t==L.backspace?s?(S.verbose("Removing active labels"),c&&u&&!d&&S.focusSearch(),i.last().next(R.siblingLabel).addClass(A.active),S.remove.activeLabels(i),e.preventDefault()):f&&!s&&t==L.backspace&&(S.verbose("Removing last label on input backspace"),i=n.last().addClass(A.active),S.remove.activeLabels(i)):i.removeClass(A.active)}}},keydown:function(e){var t=e.which;if(S.is.inObject(t,L)){var n,i=B.not(R.unselectable).filter("."+A.selected).eq(0),o=W.children("."+A.active).eq(0),a=i.length>0?i:o,s=a.length>0?a.siblings(":not(."+A.filtered+")").addBack():W.children(":not(."+A.filtered+")"),r=a.children(R.menu),l=a.closest(R.menu),c=l.hasClass(A.visible)||l.hasClass(A.animating)||l.parent(R.menu).length>0,u=r.length>0,d=a.length>0,v=a.not(R.unselectable).length>0,f=t==L.delimiter&&y.allowAdditions&&S.is.multiple(),m=y.allowAdditions&&y.hideAdditions&&(t==L.enter||f)&&v;if(m&&(S.verbose("Selecting item from keyboard shortcut",a),S.event.item.click.call(a,e),S.is.searchSelection()&&S.remove.searchTerm()),S.is.visible()){if((t==L.enter||f)&&(t==L.enter&&d&&u&&!y.allowCategorySelection?(S.verbose("Pressed enter on unselectable category, opening sub menu"),t=L.rightArrow):v&&(S.verbose("Selecting item from keyboard shortcut",a),S.event.item.click.call(a,e),S.is.searchSelection()&&S.remove.searchTerm()),e.preventDefault()),d&&(t==L.leftArrow&&l[0]!==W[0]&&(S.verbose("Left key pressed, closing sub-menu"),S.animate.hide(!1,l),a.removeClass(A.selected),l.closest(R.item).addClass(A.selected),e.preventDefault()),t==L.rightArrow&&u&&(S.verbose("Right key pressed, opening sub-menu"),S.animate.show(!1,r),a.removeClass(A.selected),r.find(R.item).eq(0).addClass(A.selected),e.preventDefault())),t==L.upArrow){if(n=d&&c?a.prevAll(R.item+":not("+R.unselectable+")").eq(0):B.eq(0),s.index(n)<0)return S.verbose("Up key pressed but reached top of current menu"),void e.preventDefault();S.verbose("Up key pressed, changing active item"),a.removeClass(A.selected),n.addClass(A.selected),S.set.scrollPosition(n),y.selectOnKeydown&&S.is.single()&&S.set.selectedItem(n),e.preventDefault()}if(t==L.downArrow){if(n=d&&c?n=a.nextAll(R.item+":not("+R.unselectable+")").eq(0):B.eq(0),0===n.length)return S.verbose("Down key pressed but reached bottom of current menu"),void e.preventDefault();S.verbose("Down key pressed, changing active item"),B.removeClass(A.selected),n.addClass(A.selected),S.set.scrollPosition(n),y.selectOnKeydown&&S.is.single()&&S.set.selectedItem(n),e.preventDefault()}t==L.pageUp&&(S.scrollPage("up"),e.preventDefault()),t==L.pageDown&&(S.scrollPage("down"),e.preventDefault()),t==L.escape&&(S.verbose("Escape key pressed, closing dropdown"),S.hide())}else f&&e.preventDefault(),t!=L.downArrow||S.is.visible()||(S.verbose("Down key pressed, showing dropdown"),S.show(),e.preventDefault())}else S.has.search()||S.set.selectedLetter(String.fromCharCode(t))}},trigger:{change:function(){var e=n.createEvent("HTMLEvents"),t=N[0];t&&(S.verbose("Triggering native change event"),e.initEvent("change",!0,!1),t.dispatchEvent(e))}},determine:{selectAction:function(t,n){S.verbose("Determining action",y.action),e.isFunction(S.action[y.action])?(S.verbose("Triggering preset action",y.action,t,n),S.action[y.action].call(Y,t,n,this)):e.isFunction(y.action)?(S.verbose("Triggering user action",y.action,t,n),y.action.call(Y,t,n,this)):S.error(O.action,y.action)},eventInModule:function(t,i){var o=e(t.target),a=o.closest(n.documentElement).length>0,s=o.closest(M).length>0;return i=e.isFunction(i)?i:function(){},a&&!s?(S.verbose("Triggering event",i),i(),!0):(S.verbose("Event occurred in dropdown, canceling callback"),!1)},eventOnElement:function(t,i){var o=e(t.target),a=o.closest(R.siblingLabel),s=n.body.contains(t.target),r=0===M.find(a).length,l=0===o.closest(W).length;return i=e.isFunction(i)?i:function(){},s&&r&&l?(S.verbose("Triggering event",i),i(),!0):(S.verbose("Event occurred in dropdown menu, canceling callback"),!1)}},action:{nothing:function(){},activate:function(t,n,i){if(n=void 0!==n?n:t,S.can.activate(e(i))){if(S.set.selected(n,e(i)),S.is.multiple()&&!S.is.allFiltered())return;S.hideAndClear()}},select:function(t,n,i){if(n=void 0!==n?n:t,S.can.activate(e(i))){if(S.set.value(n,e(i)),S.is.multiple()&&!S.is.allFiltered())return;S.hideAndClear()}},combo:function(t,n,i){n=void 0!==n?n:t,S.set.selected(n,e(i)),S.hideAndClear()},hide:function(e,t,n){S.set.value(t,e),S.hideAndClear()}},get:{id:function(){return w},defaultText:function(){return M.data(I.defaultText)},defaultValue:function(){return M.data(I.defaultValue)},placeholderText:function(){return M.data(I.placeholderText)||""},text:function(){return P.text()},query:function(){return e.trim(H.val())},searchWidth:function(e){return e=void 0!==e?e:H.val(),j.text(e),Math.ceil(j.width()+1)},selectionCount:function(){var t=S.get.values();return S.is.multiple()?Array.isArray(t)?t.length:0:""!==S.get.value()?1:0},transition:function(e){return"auto"==y.transition?S.is.upward(e)?"slide up":"slide down":y.transition},userValues:function(){var t=S.get.values();return!!t&&(t=Array.isArray(t)?t:[t],e.grep(t,function(e){return!1===S.get.item(e)}))},uniqueArray:function(t){return e.grep(t,function(n,i){return e.inArray(n,t)===i})},caretPosition:function(){var e,t,i=H.get(0);return"selectionStart"in i?i.selectionStart:n.selection?(i.focus(),e=n.selection.createRange(),t=e.text.length,e.moveStart("character",-i.value.length),e.text.length-t):void 0},value:function(){var t=N.length>0?N.val():M.data(I.value),n=Array.isArray(t)&&1===t.length&&""===t[0];return void 0===t||n?"":t},values:function(){var e=S.get.value();return""===e?"":!S.has.selectInput()&&S.is.multiple()?"string"==typeof e?e.split(y.delimiter):"":e},remoteValues:function(){var t=S.get.values(),n=!1;return t&&("string"==typeof t&&(t=[t]),e.each(t,function(e,t){var i=S.read.remoteData(t);S.verbose("Restoring value from session data",i,t),i&&(n||(n={}),n[t]=i)})),n},choiceText:function(t,n){if(n=void 0!==n?n:y.preserveHTML,t)return t.find(R.menu).length>0&&(S.verbose("Retrieving text of element with sub-menu"),t=t.clone(),t.find(R.menu).remove(),t.find(R.menuIcon).remove()),void 0!==t.data(I.text)?t.data(I.text):n?e.trim(t.html()):e.trim(t.text())},choiceValue:function(t,n){return n=n||S.get.choiceText(t),!!t&&(void 0!==t.data(I.value)?String(t.data(I.value)):"string"==typeof n?e.trim(n.toLowerCase()):String(n))},inputEvent:function(){var e=H[0];return!!e&&(void 0!==e.oninput?"input":void 0!==e.onpropertychange?"propertychange":"keyup")},selectValues:function(){var t={};return t.values=[],M.find("option").each(function(){var n=e(this),i=n.html(),o=n.attr("disabled"),a=void 0!==n.attr("value")?n.attr("value"):i;"auto"===y.placeholder&&""===a?t.placeholder=i:t.values.push({name:i,value:a,disabled:o})}),y.placeholder&&"auto"!==y.placeholder&&(S.debug("Setting placeholder value to",y.placeholder),t.placeholder=y.placeholder),y.sortSelect?(t.values.sort(function(e,t){return e.name>t.name?1:-1}),S.debug("Retrieved and sorted values from select",t)):S.debug("Retrieved values from select",t),t},activeItem:function(){return B.filter("."+A.active)},selectedItem:function(){var e=B.not(R.unselectable).filter("."+A.selected);return e.length>0?e:B.eq(0)},itemWithAdditions:function(e){var t=S.get.item(e),n=S.create.userChoice(e);return n&&n.length>0&&(t=t.length>0?t.add(n):n),t},item:function(t,n){var i,o,a=!1;return t=void 0!==t?t:void 0!==S.get.values()?S.get.values():S.get.text(),i=o?t.length>0:void 0!==t&&null!==t,o=S.is.multiple()&&Array.isArray(t),n=""===t||0===t||(n||!1),i&&B.each(function(){var i=e(this),s=S.get.choiceText(i),r=S.get.choiceValue(i,s);if(null!==r&&void 0!==r)if(o)-1===e.inArray(String(r),t)&&-1===e.inArray(s,t)||(a=a?a.add(i):i);else if(n){if(S.verbose("Ambiguous dropdown value using strict type check",i,t),r===t||s===t)return a=i,!0}else if(String(r)==String(t)||s==t)return S.verbose("Found select item by value",r,t),a=i,!0}),a}},check:{maxSelections:function(e){return!y.maxSelections||(e=void 0!==e?e:S.get.selectionCount(),e>=y.maxSelections?(S.debug("Maximum selection count reached"),y.useLabels&&(B.addClass(A.filtered),S.add.message(T.maxSelections)),!0):(S.verbose("No longer at maximum selection count"),S.remove.message(),S.remove.filteredItem(),S.is.searchSelection()&&S.filterItems(),!1))}},restore:{defaults:function(){S.clear(),S.restore.defaultText(),S.restore.defaultValue()},defaultText:function(){var e=S.get.defaultText();e===S.get.placeholderText?(S.debug("Restoring default placeholder text",e),S.set.placeholderText(e)):(S.debug("Restoring default text",e),S.set.text(e))},placeholderText:function(){S.set.placeholderText()},defaultValue:function(){var e=S.get.defaultValue();void 0!==e&&(S.debug("Restoring default value",e),""!==e?(S.set.value(e),S.set.selected()):(S.remove.activeItem(),S.remove.selectedItem()))},labels:function(){y.allowAdditions&&(y.useLabels||(S.error(O.labels),y.useLabels=!0),S.debug("Restoring selected values"),S.create.userLabels()),S.check.maxSelections()},selected:function(){S.restore.values(),S.is.multiple()?(S.debug("Restoring previously selected values and labels"),S.restore.labels()):S.debug("Restoring previously selected values")},values:function(){S.set.initialLoad(),y.apiSettings&&y.saveRemoteData&&S.get.remoteValues()?S.restore.remoteValues():S.set.selected(),S.remove.initialLoad()},remoteValues:function(){var t=S.get.remoteValues();S.debug("Recreating selected from session data",t),t&&(S.is.single()?e.each(t,function(e,t){S.set.text(t)}):e.each(t,function(e,t){S.add.label(e,t)}))}},read:{remoteData:function(e){var n;return void 0===t.Storage?void S.error(O.noStorage):void 0!==(n=sessionStorage.getItem(e))&&n}},save:{defaults:function(){S.save.defaultText(),S.save.placeholderText(),S.save.defaultValue()},defaultValue:function(){var e=S.get.value();S.verbose("Saving default value as",e),M.data(I.defaultValue,e)},defaultText:function(){var e=S.get.text();S.verbose("Saving default text as",e),M.data(I.defaultText,e)},placeholderText:function(){var e;!1!==y.placeholder&&P.hasClass(A.placeholder)&&(e=S.get.text(),S.verbose("Saving placeholder text as",e),M.data(I.placeholderText,e))},remoteData:function(e,n){if(void 0===t.Storage)return void S.error(O.noStorage);S.verbose("Saving remote data to session storage",n,e),sessionStorage.setItem(n,e)}},clear:function(){S.is.multiple()&&y.useLabels?S.remove.labels():(S.remove.activeItem(),S.remove.selectedItem()),S.set.placeholderText(),S.clearValue()},clearValue:function(){S.set.value("")},scrollPage:function(e,t){var n,i,o,a=t||S.get.selectedItem(),s=a.closest(R.menu),r=s.outerHeight(),l=s.scrollTop(),c=B.eq(0).outerHeight(),u=Math.floor(r/c),d=(s.prop("scrollHeight"),"up"==e?l-c*u:l+c*u),v=B.not(R.unselectable);o="up"==e?v.index(a)-u:v.index(a)+u,n="up"==e?o>=0:o<v.length,i=n?v.eq(o):"up"==e?v.first():v.last(),i.length>0&&(S.debug("Scrolling page",e,i),a.removeClass(A.selected),i.addClass(A.selected),y.selectOnKeydown&&S.is.single()&&S.set.selectedItem(i),s.scrollTop(d))},set:{filtered:function(){var e=S.is.multiple(),t=S.is.searchSelection(),n=e&&t,i=t?S.get.query():"",o="string"==typeof i&&i.length>0,a=S.get.searchWidth(),s=""!==i;e&&o&&(S.verbose("Adjusting input width",a,y.glyphWidth),H.css("width",a)),o||n&&s?(S.verbose("Hiding placeholder text"),P.addClass(A.filtered)):(!e||n&&!s)&&(S.verbose("Showing placeholder text"),P.removeClass(A.filtered))},empty:function(){M.addClass(A.empty)},loading:function(){M.addClass(A.loading)},placeholderText:function(e){e=e||S.get.placeholderText(),S.debug("Setting placeholder text",e),S.set.text(e),P.addClass(A.placeholder)},tabbable:function(){S.is.searchSelection()?(S.debug("Added tabindex to searchable dropdown"),H.val("").attr("tabindex",0),W.attr("tabindex",-1)):(S.debug("Added tabindex to dropdown"),void 0===M.attr("tabindex")&&(M.attr("tabindex",0),W.attr("tabindex",-1)))},initialLoad:function(){S.verbose("Setting initial load"),h=!0},activeItem:function(e){y.allowAdditions&&e.filter(R.addition).length>0?e.addClass(A.filtered):e.addClass(A.active)},partialSearch:function(e){var t=S.get.query().length;H.val(e.substr(0,t))},scrollPosition:function(e,t){var n,i,o,a,s,r,l,c,u;e=e||S.get.selectedItem(),n=e.closest(R.menu),i=e&&e.length>0,t=void 0!==t&&t,e&&n.length>0&&i&&(a=e.position().top,n.addClass(A.loading),r=n.scrollTop(),s=n.offset().top,a=e.offset().top,o=r-s+a,t||(l=n.height(),u=r+l<o+5,c=o-5<r),S.debug("Scrolling to active item",o),(t||c||u)&&n.scrollTop(o),n.removeClass(A.loading))},text:function(e){"select"!==y.action&&("combo"==y.action?(S.debug("Changing combo button text",e,K),y.preserveHTML?K.html(e):K.text(e)):(e!==S.get.placeholderText()&&P.removeClass(A.placeholder),S.debug("Changing text",e,P),P.removeClass(A.filtered),y.preserveHTML?P.html(e):P.text(e)))},selectedItem:function(e){var t=S.get.choiceValue(e),n=S.get.choiceText(e,!1),i=S.get.choiceText(e,!0);S.debug("Setting user selection to item",e),S.remove.activeItem(),S.set.partialSearch(n),S.set.activeItem(e),S.set.selected(t,e),S.set.text(i)},selectedLetter:function(t){var n,i=B.filter("."+A.selected),o=i.length>0&&S.has.firstLetter(i,t),a=!1;o&&(n=i.nextAll(B).eq(0),S.has.firstLetter(n,t)&&(a=n)),a||B.each(function(){if(S.has.firstLetter(e(this),t))return a=e(this),!1}),a&&(S.verbose("Scrolling to next value with letter",t),S.set.scrollPosition(a),i.removeClass(A.selected),a.addClass(A.selected),y.selectOnKeydown&&S.is.single()&&S.set.selectedItem(a))},direction:function(e){"auto"==y.direction?(S.remove.upward(),S.can.openDownward(e)?S.remove.upward(e):S.set.upward(e),S.is.leftward(e)||S.can.openRightward(e)||S.set.leftward(e)):"upward"==y.direction&&S.set.upward(e)},upward:function(e){(e||M).addClass(A.upward)},leftward:function(e){(e||W).addClass(A.leftward)},value:function(e,t,n){var i=S.escape.value(e),o=N.length>0,a=(S.has.value(e),S.get.values()),s=void 0!==e?String(e):e;if(o){if(!y.allowReselection&&s==a&&(S.verbose("Skipping value update already same value",e,a),!S.is.initialLoad()))return;S.is.single()&&S.has.selectInput()&&S.can.extendSelect()&&(S.debug("Adding user option",e),S.add.optionValue(e)),S.debug("Updating input value",i,a),X=!0,N.val(i),!1===y.fireOnInit&&S.is.initialLoad()?S.debug("Input native change event ignored on initial load"):S.trigger.change(),X=!1}else S.verbose("Storing value in metadata",i,N),i!==a&&M.data(I.value,s);!1===y.fireOnInit&&S.is.initialLoad()?S.verbose("No callback on initial load",y.onChange):y.onChange.call(Y,e,t,n)},active:function(){M.addClass(A.active)},multiple:function(){M.addClass(A.multiple)},visible:function(){M.addClass(A.visible)},exactly:function(e,t){S.debug("Setting selected to exact values"),S.clear(),S.set.selected(e,t)},selected:function(t,n){var i=S.is.multiple();(n=y.allowAdditions?n||S.get.itemWithAdditions(t):n||S.get.item(t))&&(S.debug("Setting selected menu item to",n),S.is.multiple()&&S.remove.searchWidth(),S.is.single()?(S.remove.activeItem(),S.remove.selectedItem()):y.useLabels&&S.remove.selectedItem(),n.each(function(){var t=e(this),o=S.get.choiceText(t),a=S.get.choiceValue(t,o),s=t.hasClass(A.filtered),r=t.hasClass(A.active),l=t.hasClass(A.addition),c=i&&1==n.length;i?!r||l?(y.apiSettings&&y.saveRemoteData&&S.save.remoteData(o,a),y.useLabels?(S.add.value(a,o,t),S.add.label(a,o,c),S.set.activeItem(t),S.filterActive(),S.select.nextAvailable(n)):(S.add.value(a,o,t),S.set.text(S.add.variables(T.count)),S.set.activeItem(t))):s||(S.debug("Selected active value, removing label"),S.remove.selected(a)):(y.apiSettings&&y.saveRemoteData&&S.save.remoteData(o,a),S.set.text(o),S.set.value(a,o,t),t.addClass(A.active).addClass(A.selected))}))}},add:{label:function(t,n,i){var o,a=S.is.searchSelection()?H:P,s=S.escape.value(t);if(o=e("<a />").addClass(A.label).attr("data-"+I.value,s).html(V.label(s,n)),o=y.onLabelCreate.call(o,s,n),S.has.label(t))return void S.debug("Label already exists, skipping",s)
;y.label.variation&&o.addClass(y.label.variation),!0===i?(S.debug("Animating in label",o),o.addClass(A.hidden).insertBefore(a).transition(y.label.transition,y.label.duration)):(S.debug("Adding selection label",o),o.insertBefore(a))},message:function(t){var n=W.children(R.message),i=y.templates.message(S.add.variables(t));n.length>0?n.html(i):n=e("<div/>").html(i).addClass(A.message).appendTo(W)},optionValue:function(t){var n=S.escape.value(t);N.find('option[value="'+S.escape.string(n)+'"]').length>0||(S.disconnect.selectObserver(),S.is.single()&&(S.verbose("Removing previous user addition"),N.find("option."+A.addition).remove()),e("<option/>").prop("value",n).addClass(A.addition).html(t).appendTo(N),S.verbose("Adding user addition as an <option>",t),S.observe.select())},userSuggestion:function(e){var t,n=W.children(R.addition),i=S.get.item(e),o=i&&i.not(R.addition).length,a=n.length>0;if(!y.useLabels||!S.has.maxSelections()){if(""===e||o)return void n.remove();a?(n.data(I.value,e).data(I.text,e).attr("data-"+I.value,e).attr("data-"+I.text,e).removeClass(A.filtered),y.hideAdditions||(t=y.templates.addition(S.add.variables(T.addResult,e)),n.html(t)),S.verbose("Replacing user suggestion with new value",n)):(n=S.create.userChoice(e),n.prependTo(W),S.verbose("Adding item choice to menu corresponding with user choice addition",n)),y.hideAdditions&&!S.is.allFiltered()||n.addClass(A.selected).siblings().removeClass(A.selected),S.refreshItems()}},variables:function(e,t){var n,i,o=-1!==e.search("{count}"),a=-1!==e.search("{maxCount}"),s=-1!==e.search("{term}");return S.verbose("Adding templated variables to message",e),o&&(n=S.get.selectionCount(),e=e.replace("{count}",n)),a&&(n=S.get.selectionCount(),e=e.replace("{maxCount}",y.maxSelections)),s&&(i=t||S.get.query(),e=e.replace("{term}",i)),e},value:function(t,n,i){var o,a=S.get.values();if(""===t)return void S.debug("Cannot select blank values from multiselect");Array.isArray(a)?(o=a.concat([t]),o=S.get.uniqueArray(o)):o=[t],S.has.selectInput()?S.can.extendSelect()&&(S.debug("Adding value to select",t,o,N),S.add.optionValue(t)):(o=o.join(y.delimiter),S.debug("Setting hidden input to delimited value",o,N)),!1===y.fireOnInit&&S.is.initialLoad()?S.verbose("Skipping onadd callback on initial load",y.onAdd):y.onAdd.call(Y,t,n,i),S.set.value(o,t,n,i),S.check.maxSelections()}},remove:{active:function(){M.removeClass(A.active)},activeLabel:function(){M.find(R.label).removeClass(A.active)},empty:function(){M.removeClass(A.empty)},loading:function(){M.removeClass(A.loading)},initialLoad:function(){h=!1},upward:function(e){(e||M).removeClass(A.upward)},leftward:function(e){(e||W).removeClass(A.leftward)},visible:function(){M.removeClass(A.visible)},activeItem:function(){B.removeClass(A.active)},filteredItem:function(){y.useLabels&&S.has.maxSelections()||(y.useLabels&&S.is.multiple()?B.not("."+A.active).removeClass(A.filtered):B.removeClass(A.filtered),S.remove.empty())},optionValue:function(e){var t=S.escape.value(e),n=N.find('option[value="'+S.escape.string(t)+'"]');n.length>0&&n.hasClass(A.addition)&&(x&&(x.disconnect(),S.verbose("Temporarily disconnecting mutation observer")),n.remove(),S.verbose("Removing user addition as an <option>",t),x&&x.observe(N[0],{childList:!0,subtree:!0}))},message:function(){W.children(R.message).remove()},searchWidth:function(){H.css("width","")},searchTerm:function(){S.verbose("Cleared search term"),H.val(""),S.set.filtered()},userAddition:function(){B.filter(R.addition).remove()},selected:function(t,n){if(!(n=y.allowAdditions?n||S.get.itemWithAdditions(t):n||S.get.item(t)))return!1;n.each(function(){var t=e(this),n=S.get.choiceText(t),i=S.get.choiceValue(t,n);S.is.multiple()?y.useLabels?(S.remove.value(i,n,t),S.remove.label(i)):(S.remove.value(i,n,t),0===S.get.selectionCount()?S.set.placeholderText():S.set.text(S.add.variables(T.count))):S.remove.value(i,n,t),t.removeClass(A.filtered).removeClass(A.active),y.useLabels&&t.removeClass(A.selected)})},selectedItem:function(){B.removeClass(A.selected)},value:function(e,t,n){var i,o=S.get.values();S.has.selectInput()?(S.verbose("Input is <select> removing selected option",e),i=S.remove.arrayValue(e,o),S.remove.optionValue(e)):(S.verbose("Removing from delimited values",e),i=S.remove.arrayValue(e,o),i=i.join(y.delimiter)),!1===y.fireOnInit&&S.is.initialLoad()?S.verbose("No callback on initial load",y.onRemove):y.onRemove.call(Y,e,t,n),S.set.value(i,t,n),S.check.maxSelections()},arrayValue:function(t,n){return Array.isArray(n)||(n=[n]),n=e.grep(n,function(e){return t!=e}),S.verbose("Removed value from delimited string",t,n),n},label:function(e,t){var n=M.find(R.label),i=n.filter("[data-"+I.value+'="'+S.escape.string(e)+'"]');S.verbose("Removing label",i),i.remove()},activeLabels:function(e){e=e||M.find(R.label).filter("."+A.active),S.verbose("Removing active label selections",e),S.remove.labels(e)},labels:function(t){t=t||M.find(R.label),S.verbose("Removing labels",t),t.each(function(){var t=e(this),n=t.data(I.value),i=void 0!==n?String(n):n,o=S.is.userValue(i);if(!1===y.onLabelRemove.call(t,n))return void S.debug("Label remove callback cancelled removal");S.remove.message(),o?(S.remove.value(i),S.remove.label(i)):S.remove.selected(i)})},tabbable:function(){S.is.searchSelection()?(S.debug("Searchable dropdown initialized"),H.removeAttr("tabindex"),W.removeAttr("tabindex")):(S.debug("Simple selection dropdown initialized"),M.removeAttr("tabindex"),W.removeAttr("tabindex"))}},has:{menuSearch:function(){return S.has.search()&&H.closest(W).length>0},search:function(){return H.length>0},sizer:function(){return j.length>0},selectInput:function(){return N.is("select")},minCharacters:function(e){return!y.minCharacters||(e=void 0!==e?String(e):String(S.get.query()),e.length>=y.minCharacters)},firstLetter:function(e,t){var n,i;return!(!e||0===e.length||"string"!=typeof t)&&(n=S.get.choiceText(e,!1),t=t.toLowerCase(),i=String(n).charAt(0).toLowerCase(),t==i)},input:function(){return N.length>0},items:function(){return B.length>0},menu:function(){return W.length>0},message:function(){return 0!==W.children(R.message).length},label:function(e){var t=S.escape.value(e);return M.find(R.label).filter("[data-"+I.value+'="'+S.escape.string(t)+'"]').length>0},maxSelections:function(){return y.maxSelections&&S.get.selectionCount()>=y.maxSelections},allResultsFiltered:function(){var e=B.not(R.addition);return e.filter(R.unselectable).length===e.length},userSuggestion:function(){return W.children(R.addition).length>0},query:function(){return""!==S.get.query()},value:function(t){var n=S.get.values();return!!(Array.isArray(n)?n&&-1!==e.inArray(t,n):n==t)}},is:{active:function(){return M.hasClass(A.active)},bubbledLabelClick:function(t){return e(t.target).is("select, input")&&M.closest("label").length>0},bubbledIconClick:function(t){return e(t.target).closest(U).length>0},alreadySetup:function(){return M.is("select")&&M.parent(R.dropdown).length>0&&0===M.prev().length},animating:function(e){return e?e.transition&&e.transition("is animating"):W.transition&&W.transition("is animating")},leftward:function(e){return(e||W).hasClass(A.leftward)},disabled:function(){return M.hasClass(A.disabled)},focused:function(){return n.activeElement===M[0]},focusedOnSearch:function(){return n.activeElement===H[0]},allFiltered:function(){return(S.is.multiple()||S.has.search())&&!(0==y.hideAdditions&&S.has.userSuggestion())&&!S.has.message()&&S.has.allResultsFiltered()},hidden:function(e){return!S.is.visible(e)},initialLoad:function(){return h},inObject:function(t,n){var i=!1;return e.each(n,function(e,n){if(n==t)return i=!0,!0}),i},multiple:function(){return M.hasClass(A.multiple)},remote:function(){return y.apiSettings&&S.can.useAPI()},single:function(){return!S.is.multiple()},selectMutation:function(t){var n=!1;return e.each(t,function(t,i){if(i.target&&e(i.target).is("select"))return n=!0,!0}),n},search:function(){return M.hasClass(A.search)},searchSelection:function(){return S.has.search()&&1===H.parent(R.dropdown).length},selection:function(){return M.hasClass(A.selection)},userValue:function(t){return-1!==e.inArray(t,S.get.userValues())},upward:function(e){return(e||M).hasClass(A.upward)},visible:function(e){return e?e.hasClass(A.visible):W.hasClass(A.visible)},verticallyScrollableContext:function(){var e=z.get(0)!==t&&z.css("overflow-y");return"auto"==e||"scroll"==e},horizontallyScrollableContext:function(){var e=z.get(0)!==t&&z.css("overflow-X");return"auto"==e||"scroll"==e}},can:{activate:function(e){return!!y.useLabels||(!S.has.maxSelections()||!(!S.has.maxSelections()||!e.hasClass(A.active)))},openDownward:function(e){var t,n=e||W,i=!0,o={};return n.addClass(A.loading),t={context:{scrollTop:z.scrollTop(),height:z.outerHeight()},menu:{offset:n.offset(),height:n.outerHeight()}},S.is.verticallyScrollableContext()&&(t.menu.offset.top+=t.context.scrollTop),o={above:t.context.scrollTop<=t.menu.offset.top-t.menu.height,below:t.context.scrollTop+t.context.height>=t.menu.offset.top+t.menu.height},o.below?(S.verbose("Dropdown can fit in context downward",o),i=!0):o.below||o.above?(S.verbose("Dropdown cannot fit below, opening upward",o),i=!1):(S.verbose("Dropdown cannot fit in either direction, favoring downward",o),i=!0),n.removeClass(A.loading),i},openRightward:function(e){var t,n=e||W,i=!0,o=!1;return n.addClass(A.loading),t={context:{scrollLeft:z.scrollLeft(),width:z.outerWidth()},menu:{offset:n.offset(),width:n.outerWidth()}},S.is.horizontallyScrollableContext()&&(t.menu.offset.left+=t.context.scrollLeft),o=t.menu.offset.left+t.menu.width>=t.context.scrollLeft+t.context.width,o&&(S.verbose("Dropdown cannot fit in context rightward",o),i=!1),n.removeClass(A.loading),i},click:function(){return l||"click"==y.on},extendSelect:function(){return y.allowAdditions||y.apiSettings},show:function(){return!S.is.disabled()&&(S.has.items()||S.has.message())},useAPI:function(){return void 0!==e.fn.api}},animate:{show:function(t,n){var i,o=n||W,a=n?function(){}:function(){S.hideSubMenus(),S.hideOthers(),S.set.active()};t=e.isFunction(t)?t:function(){},S.verbose("Doing menu show animation",o),S.set.direction(n),i=S.get.transition(n),S.is.selection()&&S.set.scrollPosition(S.get.selectedItem(),!0),(S.is.hidden(o)||S.is.animating(o))&&("none"==i?(a(),o.transition("show"),t.call(Y)):void 0!==e.fn.transition&&M.transition("is supported")?o.transition({animation:i+" in",debug:y.debug,verbose:y.verbose,duration:y.duration,queue:!0,onStart:a,onComplete:function(){t.call(Y)}}):S.error(O.noTransition,i))},hide:function(t,n){var i=n||W,o=(n?y.duration:y.duration,n?function(){}:function(){S.can.click()&&S.unbind.intent(),S.remove.active()}),a=S.get.transition(n);t=e.isFunction(t)?t:function(){},(S.is.visible(i)||S.is.animating(i))&&(S.verbose("Doing menu hide animation",i),"none"==a?(o(),i.transition("hide"),t.call(Y)):void 0!==e.fn.transition&&M.transition("is supported")?i.transition({animation:a+" out",duration:y.duration,debug:y.debug,verbose:y.verbose,queue:!0,onStart:o,onComplete:function(){t.call(Y)}}):S.error(O.transition))}},hideAndClear:function(){S.remove.searchTerm(),S.has.maxSelections()||(S.has.search()?S.hide(function(){S.remove.filteredItem()}):S.hide())},delay:{show:function(){S.verbose("Delaying show event to ensure user intent"),clearTimeout(S.timer),S.timer=setTimeout(S.show,y.delay.show)},hide:function(){S.verbose("Delaying hide event to ensure user intent"),clearTimeout(S.timer),S.timer=setTimeout(S.hide,y.delay.hide)}},escape:{value:function(t){var n=Array.isArray(t),i="string"==typeof t,o=!i&&!n,a=i&&-1!==t.search(q.quote),s=[];return o||!a?t:(S.debug("Encoding quote values for use in select",t),n?(e.each(t,function(e,t){s.push(t.replace(q.quote,"&quot;"))}),s):t.replace(q.quote,"&quot;"))},string:function(e){return e=String(e),e.replace(q.escape,"\\$&")}},setting:function(t,n){if(S.debug("Changing setting",t,n),e.isPlainObject(t))e.extend(!0,y,t);else{if(void 0===n)return y[t];e.isPlainObject(y[t])?e.extend(!0,y[t],n):y[t]=n}},internal:function(t,n){if(e.isPlainObject(t))e.extend(!0,S,t);else{if(void 0===n)return S[t];S[t]=n}},debug:function(){!y.silent&&y.debug&&(y.performance?S.performance.log(arguments):(S.debug=Function.prototype.bind.call(console.info,console,y.name+":"),S.debug.apply(console,arguments)))},verbose:function(){!y.silent&&y.verbose&&y.debug&&(y.performance?S.performance.log(arguments):(S.verbose=Function.prototype.bind.call(console.info,console,y.name+":"),S.verbose.apply(console,arguments)))},error:function(){y.silent||(S.error=Function.prototype.bind.call(console.error,console,y.name+":"),S.error.apply(console,arguments))},performance:{log:function(e){var t,n,i;y.performance&&(t=(new Date).getTime(),i=c||t,n=t-i,c=t,u.push({Name:e[0],Arguments:[].slice.call(e,1)||"",Element:Y,"Execution Time":n})),clearTimeout(S.performance.timer),S.performance.timer=setTimeout(S.performance.display,500)},display:function(){var t=y.name+":",n=0;c=!1,clearTimeout(S.performance.timer),e.each(u,function(e,t){n+=t["Execution Time"]}),t+=" "+n+"ms",r&&(t+=" '"+r+"'"),(void 0!==console.group||void 0!==console.table)&&u.length>0&&(console.groupCollapsed(t),console.table?console.table(u):e.each(u,function(e,t){console.log(t.Name+": "+t["Execution Time"]+"ms")}),console.groupEnd()),u=[]}},invoke:function(t,n,i){var a,s,r,l=G;return n=n||f,i=Y||i,"string"==typeof t&&void 0!==l&&(t=t.split(/[\. ]/),a=t.length-1,e.each(t,function(n,i){var o=n!=a?i+t[n+1].charAt(0).toUpperCase()+t[n+1].slice(1):t;if(e.isPlainObject(l[o])&&n!=a)l=l[o];else{if(void 0!==l[o])return s=l[o],!1;if(!e.isPlainObject(l[i])||n==a)return void 0!==l[i]?(s=l[i],!1):(S.error(O.method,t),!1);l=l[i]}})),e.isFunction(s)?r=s.apply(i,n):void 0!==s&&(r=s),Array.isArray(o)?o.push(r):void 0!==o?o=[o,r]:void 0!==r&&(o=r),s}},v?(void 0===G&&S.initialize(),S.invoke(d)):(void 0!==G&&G.invoke("destroy"),S.initialize())}),void 0!==o?o:a},e.fn.dropdown.settings={silent:!1,debug:!1,verbose:!1,performance:!0,on:"click",action:"activate",apiSettings:!1,selectOnKeydown:!0,minCharacters:0,filterRemoteData:!1,saveRemoteData:!0,throttle:200,context:t,direction:"auto",keepOnScreen:!0,match:"both",fullTextSearch:!1,placeholder:"auto",preserveHTML:!0,sortSelect:!1,forceSelection:!0,allowAdditions:!1,hideAdditions:!0,maxSelections:!1,useLabels:!0,delimiter:",",showOnFocus:!0,allowReselection:!1,allowTab:!0,allowCategorySelection:!1,fireOnInit:!1,transition:"auto",duration:200,glyphWidth:1.037,label:{transition:"scale",duration:200,variation:!1},delay:{hide:300,show:200,search:20,touch:50},onChange:function(e,t,n){},onAdd:function(e,t,n){},onRemove:function(e,t,n){},onLabelSelect:function(e){},onLabelCreate:function(t,n){return e(this)},onLabelRemove:function(e){return!0},onNoResults:function(e){return!0},onShow:function(){},onHide:function(){},name:"Dropdown",namespace:"dropdown",message:{addResult:"Add <b>{term}</b>",count:"{count} selected",maxSelections:"Max {maxCount} selections",noResults:"No results found.",serverError:"There was an error contacting the server"},error:{action:"You called a dropdown action that was not defined",alreadySetup:"Once a select has been initialized behaviors must be called on the created ui dropdown",labels:"Allowing user additions currently requires the use of labels.",missingMultiple:"<select> requires multiple property to be set to correctly preserve multiple values",method:"The method you called is not defined.",noAPI:"The API module is required to load resources remotely",noStorage:"Saving remote data requires session storage",noTransition:"This module requires ui transitions <https://github.com/Semantic-Org/UI-Transition>"},regExp:{escape:/[-[\]{}()*+?.,\\^$|#\s]/g,quote:/"/g},metadata:{defaultText:"defaultText",defaultValue:"defaultValue",placeholderText:"placeholder",text:"text",value:"value"},fields:{remoteValues:"results",values:"values",disabled:"disabled",name:"name",value:"value",text:"text"},keys:{backspace:8,delimiter:188,deleteKey:46,enter:13,escape:27,pageUp:33,pageDown:34,leftArrow:37,upArrow:38,rightArrow:39,downArrow:40},selector:{addition:".addition",dropdown:".ui.dropdown",hidden:".hidden",icon:"> .dropdown.icon",input:'> input[type="hidden"], > select',item:".item",label:"> .label",remove:"> .label > .delete.icon",siblingLabel:".label",menu:".menu",message:".message",menuIcon:".dropdown.icon",search:"input.search, .menu > .search > input, .menu input.search",sizer:"> input.sizer",text:"> .text:not(.icon)",unselectable:".disabled, .filtered"},className:{active:"active",addition:"addition",animating:"animating",disabled:"disabled",empty:"empty",dropdown:"ui dropdown",filtered:"filtered",hidden:"hidden transition",item:"item",label:"ui label",loading:"loading",menu:"menu",message:"message",multiple:"multiple",placeholder:"default",sizer:"sizer",search:"search",selected:"selected",selection:"selection",upward:"upward",leftward:"left",visible:"visible"}},e.fn.dropdown.settings.templates={dropdown:function(t){var n=t.placeholder||!1,i=(t.values,"");return i+='<i class="dropdown icon"></i>',t.placeholder?i+='<div class="default text">'+n+"</div>":i+='<div class="text"></div>',i+='<div class="menu">',e.each(t.values,function(e,t){i+=t.disabled?'<div class="disabled item" data-value="'+t.value+'">'+t.name+"</div>":'<div class="item" data-value="'+t.value+'">'+t.name+"</div>"}),i+="</div>"},menu:function(t,n){var i=t[n.values]||{},o="";return e.each(i,function(e,t){var i=t[n.text]?'data-text="'+t[n.text]+'"':"",a=t[n.disabled]?"disabled ":"";o+='<div class="'+a+'item" data-value="'+t[n.value]+'"'+i+">",o+=t[n.name],o+="</div>"}),o},label:function(e,t){return t+'<i class="delete icon"></i>'},message:function(e){return e},addition:function(e){return e}}}(jQuery,window,document);/*!
 * # Semantic UI 2.2.11 - Transition
 * http://github.com/semantic-org/semantic-ui/
 *
 *
 * Released under the MIT license
 * http://opensource.org/licenses/MIT
 *
 */
!function(n,i,e,t){"use strict";i=void 0!==i&&i.Math==Math?i:"undefined"!=typeof self&&self.Math==Math?self:Function("return this")(),n.fn.transition=function(){var t,a=n(this),o=a.selector||"",r=(new Date).getTime(),s=[],l=arguments,d=l[0],u=[].slice.call(arguments,1),c="string"==typeof d;i.requestAnimationFrame||i.mozRequestAnimationFrame||i.webkitRequestAnimationFrame||i.msRequestAnimationFrame;return a.each(function(i){var m,f,p,g,v,b,y,h,w,C=n(this),A=this;w={initialize:function(){m=w.get.settings.apply(A,l),g=m.className,p=m.error,v=m.metadata,h="."+m.namespace,y="module-"+m.namespace,f=C.data(y)||w,b=w.get.animationEndEvent(),c&&(c=w.invoke(d)),!1===c&&(w.verbose("Converted arguments into settings object",m),m.interval?w.delay(m.animate):w.animate(),w.instantiate())},instantiate:function(){w.verbose("Storing instance of module",w),f=w,C.data(y,f)},destroy:function(){w.verbose("Destroying previous module for",A),C.removeData(y)},refresh:function(){w.verbose("Refreshing display type on next animation"),delete w.displayType},forceRepaint:function(){w.verbose("Forcing element repaint");var n=C.parent(),i=C.next();0===i.length?C.detach().appendTo(n):C.detach().insertBefore(i)},repaint:function(){w.verbose("Repainting element");A.offsetWidth},delay:function(n){var e,t,o=w.get.animationDirection();o||(o=w.can.transition()?w.get.direction():"static"),n=void 0!==n?n:m.interval,e="auto"==m.reverse&&o==g.outward,t=e||1==m.reverse?(a.length-i)*m.interval:i*m.interval,w.debug("Delaying animation by",t),setTimeout(w.animate,t)},animate:function(n){if(m=n||m,!w.is.supported())return w.error(p.support),!1;if(w.debug("Preparing animation",m.animation),w.is.animating()){if(m.queue)return!m.allowRepeats&&w.has.direction()&&w.is.occurring()&&!0!==w.queuing?w.debug("Animation is currently occurring, preventing queueing same animation",m.animation):w.queue(m.animation),!1;if(!m.allowRepeats&&w.is.occurring())return w.debug("Animation is already occurring, will not execute repeated animation",m.animation),!1;w.debug("New animation started, completing previous early",m.animation),f.complete()}w.can.animate()?w.set.animating(m.animation):w.error(p.noAnimation,m.animation,A)},reset:function(){w.debug("Resetting animation to beginning conditions"),w.remove.animationCallbacks(),w.restore.conditions(),w.remove.animating()},queue:function(n){w.debug("Queueing animation of",n),w.queuing=!0,C.one(b+".queue"+h,function(){w.queuing=!1,w.repaint(),w.animate.apply(this,m)})},complete:function(n){w.debug("Animation complete",m.animation),w.remove.completeCallback(),w.remove.failSafe(),w.is.looping()||(w.is.outward()?(w.verbose("Animation is outward, hiding element"),w.restore.conditions(),w.hide()):w.is.inward()?(w.verbose("Animation is outward, showing element"),w.restore.conditions(),w.show()):(w.verbose("Static animation completed"),w.restore.conditions(),m.onComplete.call(A)))},force:{visible:function(){var n=C.attr("style"),i=w.get.userStyle(),e=w.get.displayType(),t=i+"display: "+e+" !important;",a=C.css("display"),o=void 0===n||""===n;a!==e?(w.verbose("Overriding default display to show element",e),C.attr("style",t)):o&&C.removeAttr("style")},hidden:function(){var n=C.attr("style"),i=C.css("display"),e=void 0===n||""===n;"none"===i||w.is.hidden()?e&&C.removeAttr("style"):(w.verbose("Overriding default display to hide element"),C.css("display","none"))}},has:{direction:function(i){var e=!1;return i=i||m.animation,"string"==typeof i&&(i=i.split(" "),n.each(i,function(n,i){i!==g.inward&&i!==g.outward||(e=!0)})),e},inlineDisplay:function(){var i=C.attr("style")||"";return n.isArray(i.match(/display.*?;/,""))}},set:{animating:function(n){var i;w.remove.completeCallback(),n=n||m.animation,i=w.get.animationClass(n),w.save.animation(i),w.force.visible(),w.remove.hidden(),w.remove.direction(),w.start.animation(i)},duration:function(n,i){i=i||m.duration,((i="number"==typeof i?i+"ms":i)||0===i)&&(w.verbose("Setting animation duration",i),C.css({"animation-duration":i}))},direction:function(n){n=n||w.get.direction(),n==g.inward?w.set.inward():w.set.outward()},looping:function(){w.debug("Transition set to loop"),C.addClass(g.looping)},hidden:function(){C.addClass(g.transition).addClass(g.hidden)},inward:function(){w.debug("Setting direction to inward"),C.removeClass(g.outward).addClass(g.inward)},outward:function(){w.debug("Setting direction to outward"),C.removeClass(g.inward).addClass(g.outward)},visible:function(){C.addClass(g.transition).addClass(g.visible)}},start:{animation:function(n){n=n||w.get.animationClass(),w.debug("Starting tween",n),C.addClass(n).one(b+".complete"+h,w.complete),m.useFailSafe&&w.add.failSafe(),w.set.duration(m.duration),m.onStart.call(A)}},save:{animation:function(n){w.cache||(w.cache={}),w.cache.animation=n},displayType:function(n){"none"!==n&&C.data(v.displayType,n)},transitionExists:function(i,e){n.fn.transition.exists[i]=e,w.verbose("Saving existence of transition",i,e)}},restore:{conditions:function(){var n=w.get.currentAnimation();n&&(C.removeClass(n),w.verbose("Removing animation class",w.cache)),w.remove.duration()}},add:{failSafe:function(){var n=w.get.duration();w.timer=setTimeout(function(){C.triggerHandler(b)},n+m.failSafeDelay),w.verbose("Adding fail safe timer",w.timer)}},remove:{animating:function(){C.removeClass(g.animating)},animationCallbacks:function(){w.remove.queueCallback(),w.remove.completeCallback()},queueCallback:function(){C.off(".queue"+h)},completeCallback:function(){C.off(".complete"+h)},display:function(){C.css("display","")},direction:function(){C.removeClass(g.inward).removeClass(g.outward)},duration:function(){C.css("animation-duration","")},failSafe:function(){w.verbose("Removing fail safe timer",w.timer),w.timer&&clearTimeout(w.timer)},hidden:function(){C.removeClass(g.hidden)},visible:function(){C.removeClass(g.visible)},looping:function(){w.debug("Transitions are no longer looping"),w.is.looping()&&(w.reset(),C.removeClass(g.looping))},transition:function(){C.removeClass(g.visible).removeClass(g.hidden)}},get:{settings:function(i,e,t){return"object"==typeof i?n.extend(!0,{},n.fn.transition.settings,i):"function"==typeof t?n.extend({},n.fn.transition.settings,{animation:i,onComplete:t,duration:e}):"string"==typeof e||"number"==typeof e?n.extend({},n.fn.transition.settings,{animation:i,duration:e}):"object"==typeof e?n.extend({},n.fn.transition.settings,e,{animation:i}):"function"==typeof e?n.extend({},n.fn.transition.settings,{animation:i,onComplete:e}):n.extend({},n.fn.transition.settings,{animation:i})},animationClass:function(n){var i=n||m.animation,e=w.can.transition()&&!w.has.direction()?w.get.direction()+" ":"";return g.animating+" "+g.transition+" "+e+i},currentAnimation:function(){return!(!w.cache||void 0===w.cache.animation)&&w.cache.animation},currentDirection:function(){return w.is.inward()?g.inward:g.outward},direction:function(){return w.is.hidden()||!w.is.visible()?g.inward:g.outward},animationDirection:function(i){var e;return i=i||m.animation,"string"==typeof i&&(i=i.split(" "),n.each(i,function(n,i){i===g.inward?e=g.inward:i===g.outward&&(e=g.outward)})),e||!1},duration:function(n){return n=n||m.duration,!1===n&&(n=C.css("animation-duration")||0),"string"==typeof n?n.indexOf("ms")>-1?parseFloat(n):1e3*parseFloat(n):n},displayType:function(n){return n=void 0===n||n,m.displayType?m.displayType:(n&&void 0===C.data(v.displayType)&&w.can.transition(!0),C.data(v.displayType))},userStyle:function(n){return n=n||C.attr("style")||"",n.replace(/display.*?;/,"")},transitionExists:function(i){return n.fn.transition.exists[i]},animationStartEvent:function(){var n,i=e.createElement("div"),t={animation:"animationstart",OAnimation:"oAnimationStart",MozAnimation:"mozAnimationStart",WebkitAnimation:"webkitAnimationStart"};for(n in t)if(void 0!==i.style[n])return t[n];return!1},animationEndEvent:function(){var n,i=e.createElement("div"),t={animation:"animationend",OAnimation:"oAnimationEnd",MozAnimation:"mozAnimationEnd",WebkitAnimation:"webkitAnimationEnd"};for(n in t)if(void 0!==i.style[n])return t[n];return!1}},can:{transition:function(i){var e,t,a,o,r,s,l=m.animation,d=w.get.transitionExists(l),u=w.get.displayType(!1);if(void 0===d||i){if(w.verbose("Determining whether animation exists"),e=C.attr("class"),t=C.prop("tagName"),a=n("<"+t+" />").addClass(e).insertAfter(C),o=a.addClass(l).removeClass(g.inward).removeClass(g.outward).addClass(g.animating).addClass(g.transition).css("animationName"),r=a.addClass(g.inward).css("animationName"),u||(u=a.attr("class",e).removeAttr("style").removeClass(g.hidden).removeClass(g.visible).show().css("display"),w.verbose("Determining final display state",u),w.save.displayType(u)),a.remove(),o!=r)w.debug("Direction exists for animation",l),s=!0;else{if("none"==o||!o)return void w.debug("No animation defined in css",l);w.debug("Static animation found",l,u),s=!1}w.save.transitionExists(l,s)}return void 0!==d?d:s},animate:function(){return void 0!==w.can.transition()}},is:{animating:function(){return C.hasClass(g.animating)},inward:function(){return C.hasClass(g.inward)},outward:function(){return C.hasClass(g.outward)},looping:function(){return C.hasClass(g.looping)},occurring:function(n){return n=n||m.animation,n="."+n.replace(" ","."),C.filter(n).length>0},visible:function(){return C.is(":visible")},hidden:function(){return"hidden"===C.css("visibility")},supported:function(){return!1!==b}},hide:function(){w.verbose("Hiding element"),w.is.animating()&&w.reset(),A.blur(),w.remove.display(),w.remove.visible(),w.set.hidden(),w.force.hidden(),m.onHide.call(A),m.onComplete.call(A)},show:function(n){w.verbose("Showing element",n),w.remove.hidden(),w.set.visible(),w.force.visible(),m.onShow.call(A),m.onComplete.call(A)},toggle:function(){w.is.visible()?w.hide():w.show()},stop:function(){w.debug("Stopping current animation"),C.triggerHandler(b)},stopAll:function(){w.debug("Stopping all animation"),w.remove.queueCallback(),C.triggerHandler(b)},clear:{queue:function(){w.debug("Clearing animation queue"),w.remove.queueCallback()}},enable:function(){w.verbose("Starting animation"),C.removeClass(g.disabled)},disable:function(){w.debug("Stopping animation"),C.addClass(g.disabled)},setting:function(i,e){if(w.debug("Changing setting",i,e),n.isPlainObject(i))n.extend(!0,m,i);else{if(void 0===e)return m[i];n.isPlainObject(m[i])?n.extend(!0,m[i],e):m[i]=e}},internal:function(i,e){if(n.isPlainObject(i))n.extend(!0,w,i);else{if(void 0===e)return w[i];w[i]=e}},debug:function(){!m.silent&&m.debug&&(m.performance?w.performance.log(arguments):(w.debug=Function.prototype.bind.call(console.info,console,m.name+":"),w.debug.apply(console,arguments)))},verbose:function(){!m.silent&&m.verbose&&m.debug&&(m.performance?w.performance.log(arguments):(w.verbose=Function.prototype.bind.call(console.info,console,m.name+":"),w.verbose.apply(console,arguments)))},error:function(){m.silent||(w.error=Function.prototype.bind.call(console.error,console,m.name+":"),w.error.apply(console,arguments))},performance:{log:function(n){var i,e,t;m.performance&&(i=(new Date).getTime(),t=r||i,e=i-t,r=i,s.push({Name:n[0],Arguments:[].slice.call(n,1)||"",Element:A,"Execution Time":e})),clearTimeout(w.performance.timer),w.performance.timer=setTimeout(w.performance.display,500)},display:function(){var i=m.name+":",e=0;r=!1,clearTimeout(w.performance.timer),n.each(s,function(n,i){e+=i["Execution Time"]}),i+=" "+e+"ms",o&&(i+=" '"+o+"'"),a.length>1&&(i+=" ("+a.length+")"),(void 0!==console.group||void 0!==console.table)&&s.length>0&&(console.groupCollapsed(i),console.table?console.table(s):n.each(s,function(n,i){console.log(i.Name+": "+i["Execution Time"]+"ms")}),console.groupEnd()),s=[]}},invoke:function(i,e,a){var o,r,s,l=f;return e=e||u,a=A||a,"string"==typeof i&&void 0!==l&&(i=i.split(/[\. ]/),o=i.length-1,n.each(i,function(e,t){var a=e!=o?t+i[e+1].charAt(0).toUpperCase()+i[e+1].slice(1):i;if(n.isPlainObject(l[a])&&e!=o)l=l[a];else{if(void 0!==l[a])return r=l[a],!1;if(!n.isPlainObject(l[t])||e==o)return void 0!==l[t]&&(r=l[t],!1);l=l[t]}})),n.isFunction(r)?s=r.apply(a,e):void 0!==r&&(s=r),n.isArray(t)?t.push(s):void 0!==t?t=[t,s]:void 0!==s&&(t=s),void 0!==r&&r}},w.initialize()}),void 0!==t?t:this},n.fn.transition.exists={},n.fn.transition.settings={name:"Transition",silent:!1,debug:!1,verbose:!1,performance:!0,namespace:"transition",interval:0,reverse:"auto",onStart:function(){},onComplete:function(){},onShow:function(){},onHide:function(){},useFailSafe:!0,failSafeDelay:100,allowRepeats:!1,displayType:!1,animation:"fade",duration:!1,queue:!0,metadata:{displayType:"display"},className:{animating:"animating",disabled:"disabled",hidden:"hidden",inward:"in",loading:"loading",looping:"looping",outward:"out",transition:"transition",visible:"visible"},error:{noAnimation:"Element is no longer attached to DOM. Unable to animate.  Use silent setting to surpress this warning in production.",repeated:"That animation is already occurring, cancelling repeated animation",method:"The method you called is not defined",support:"This browser does not support CSS animations"}}}(jQuery,window,document);$(document.body).ready(function() {
    /* Init Combobox based on Semantic UI theme */
    $('.ui.dropdown').dropdown();

    $(".search_dictionary_selector").change(function() {
        $(".search_form").attr("action", "https://www.ldoceonline.com/search/" + $(this).val() + "/direct/");
    }).change();

    var shareLink = document.URL;
    $(".share_panel_linkedin").click(function() {
        window.open("https://uk.linkedin.com/showcase/pearsonlanguages/");
    });
    $(".share_panel_youtube").click(function() {
        window.open("https://www.youtube.com/user/PearsonLongmanELT");
    });

    initAutocomplete();
    $(".search_dictionary_selector").change(function() {
        initAutocomplete();
    });

    //custom select
    $(".custom-select").each(function() {
        var o = this;
        o.elem = $(o);
        o.cslc = $(".custom-select-label-container");
        o.csm = $(".custom-select-menu");
        o.csl = $(".res_hos");
        o.si = $(".search_input");
        o.select = $(".search_dictionary_selector");

        // refresh
        o.refresh = function() {
            var val = o.elem.val();
            o.csm.find(".active.selected").removeClass("active selected");
            o.csm.find("a[data-value='" + val + "']").addClass("active selected");
            var placeholder = o.csm.find("a[data-value='" + val + "']").attr("data-placeholder") || o.csm.find("a[data-value='" + val + "']").html();
            o.si.attr('placeholder', placeholder);
            if (o.csm.find("a[data-value='" + val + "']").attr('data-specialchar')) {
                $(".search-keyboard").show();
                $(".search-keyboard").attr('data-specialchar', o.csm.find("a[data-value='" + val + "']").attr('data-specialchar'));
            } else {
                $(".search-keyboard").hide();
                $(".search-keyboard").removeAttr('data-specialchar');
            }
            if ($(".specialchar").length > 0) {
                $(".specialchar").remove();
                if ($(".search-keyboard").is(":visible") === true)
                    $(".search-keyboard").click();
            }
        };
        o.elem.change(function() {
            o.refresh();
        });
        o.refresh();

        // select
        o.csm.find("a").click(function() {
            o.select.val($(this).attr("data-value"));
            o.select.change();
        });
    });

    // PLLDOCE-448 - Add cursor to the searchbox when landing on any page
    $('.search_input').select();

    $(".custom-select-label-container").on("click mousedown mouseup touchstart", function(e) {
        //console.log(e);
        if ($(".inputSuggestions").is(":visible"))
            $(".inputSuggestions").hide();

        e.stopPropagation();
    });

    $("body").mousedown(function(e) {
        var target = $(e.toElement) || $(e.relatedTarget) || $(e.target);
        if (target.hasClass("autoc-result"))
            target.trigger("click");
    });

    $('.search-form').on('submit', function() {

        // ignore empty search
        var q = $(this).find("input[name='q']").val();
        if (q.trim() == "")
            return false;

    });

    function initAutocomplete() {
        $(".search_form").each(function() {
            var frm = $(this).get(0);
            var dictCode = $(frm).find(".search_dictionary_selector").val();
            $(frm).find(".search_input").autoCompleter({
                url: "/autocomplete/" + dictCode + "/",
                minChars: 2,
                autocompleterClass: "inputSuggestions",
                autocompleterResultClass: "suggestionResult",
                confirmSuggestionCallback: function(row) {
                    $(frm).submit();
                },
                queryCallback: function(callback) {
                    var dictCode = $(frm).find(".search_dictionary_selector").val();

                    var url = "/autocomplete/" + dictCode + "/";
                    var criterion = $(frm).find('.search_input').val();
                    var params = {
                        q: criterion,
                        contentType: 'application/json; charset=utf-8'
                    };
                    $.getJSON(url, params, function(data) {
                        callback(data);
                    });
                },
                createResultRowCallback: function(x, y) {
                    return "<li><a class='suggestionResult' data-value='" + x + "'>" + y.searchtext + "</a></li>";
                },
                footerLink: "All results",
                footerLinkCallback: function() {
                    var d = $(frm).find(".search_dictionary_selector").val();
                    $(frm).attr("action", "/search/" + d + "/");
                    $(frm).submit();
                },
                closeAfterSelect: false
            });
            $(frm).submit(function(){
                if($(this).find(".search_input").val() == "")
                    return false;
            });
        });
    }

    $(".right_box").on("click", function(e) {
        var category = $(this).hasClass("exercises") ? "exercise" : $(this).hasClass("quizzes") ? "quiz" :"";

        if (category != "") {
            var value = $(this).parents().hasClass("home") ? "homepage" : "entry";
            sendGaEvent(category, 'hook', value);
        }
    });
});

function viewTenses(tableClass, tenseDisp, viewMDisp, viewLDisp) {
    $("." + tableClass + " .next_tenses").css("display", tenseDisp);
    $("." + tableClass + " .view_more").css("display", viewMDisp);
    $("." + tableClass + " .view_less").css("display", viewLDisp);
}//sound.js
$(document).ready(function(){
    var audio = null;

    $(".speaker").click(function(){
        var src_mp3 = $(this).attr("data-src-mp3");

        if (supportAudioHtml5())
            playHtml5(src_mp3);
        else if (supportAudioFlash())
            playFlash(src_mp3);
        else
            playRaw(src_mp3);
    });

    function supportAudioHtml5(){
        var audioTag  = document.createElement('audio');
        try {
            return ( !!(audioTag.canPlayType)
                     && (audioTag.canPlayType("audio/mpeg") != "no" && audioTag.canPlayType("audio/mpeg") != "" ) );
        } catch(e){
            return false;
        } 
    }

    function supportAudioFlash() {
        var flashinstalled = 0;
        var flashversion = 0;
        if (navigator.plugins && navigator.plugins.length){
            x = navigator.plugins["Shockwave Flash"];
            if (x){
                flashinstalled = 2;
                if (x.description) {
                    y = x.description;
                    flashversion = y.charAt(y.indexOf('.')-1);
                }
            } else {
                flashinstalled = 1;
            }
            if (navigator.plugins["Shockwave Flash 2.0"]){
                flashinstalled = 2;
                flashversion = 2;
            }
        } else if (navigator.mimeTypes && navigator.mimeTypes.length){
            x = navigator.mimeTypes['application/x-shockwave-flash'];
            if (x && x.enabledPlugin)
                flashinstalled = 2;
            else
                flashinstalled = 1;
        } else {
            for(var i=7; i>0; i--){
                flashVersion = 0;
                try{
                    var flash = new ActiveXObject("ShockwaveFlash.ShockwaveFlash." + i);
                    flashVersion = i;
                    return (flashVersion > 0);
                } catch(e){}
            }
        }
        return (flashinstalled > 0);
    }

    function playHtml5(src_mp3) {
        if(audio != null){

            if(!audio.ended){
                audio.pause();
                if(audio.currentTime > 0) audio.currentTime = 0;
            }
        }

        //use appropriate source
        audio = new Audio("");
        if (audio.canPlayType("audio/mpeg") != "no" && audio.canPlayType("audio/mpeg") != "")
            audio = new Audio(src_mp3);

        //play
        audio.addEventListener("error", function(e){alert("Apologies, the sound is not available.");});
        audio.play();
    }

    function playFlash(src_mp3) {
        var src_flash ="https://www.ldoceonline.com/external/flash/speaker.swf?song_url=%22 + src_mp3 + %22&autoplay=true&version=1.2.71";
        if (navigator.plugins && navigator.mimeTypes && navigator.mimeTypes.length)
            $("body").append("<embed type='application/x-shockwave-flash' src='" + src_flash + "' width='0' height='0'></embed>");
        else
            $("body").append("<object type='application/x-shockwave-flash' width='0' height='0' codebase='https://download.macromedia.com/pub/shockwave/cabs/flash/swflash.cab#version=6,0,40,0' data='" + src_flash + "'><param name='wmode' value='transparent'/><param name='movie' value='" + src_flash + "'/><embed src='" + src_flash + "' width='0' height='0' ></embed></object>");
    }

    function playRaw(src_mp3) {
        window.open(src_mp3, "Sound", "menubar=no, status=no, scrollbars=no, menubar=no, width=200, height=100");
    }
});

/* cookie management */
function getCookie(cname){
    var regexp = new RegExp('(?:^|; )' + cname + '=([^;]*)(?:$|; )');
    var match = document.cookie.match(regexp);
    if (match) {
        var rawValue = match[1];
        try {
            var decodedValue = decodeURIComponent(rawValue);
            return decodedValue;
        } catch(e) {}
    }
    return "";
}

function getCookieInt(cName) {
    var cookieVal = getCookie(cName);
    return isNaN(cookieVal) ? 0 : parseInt(cookieVal, 10);
}

function setCookie(cName, value) {
    setCookie(cName, value, null, null, null);
}

function setCookie(cName, value, expireInDay) {
    setCookie(cName, value, expireInDay, null, null);
}

function setCookie(cName, value, expireInDay, cDomain, cPath) {
    var today = new Date();
    var expire = new Date();
    expire.setTime(today.getTime() + (1000 * 3600 * 24 * (expireInDay == null ? 365 : expireInDay)));

    var domain = (cDomain == null ? "" : ";domain=" + cDomain);
    var path = ";path=" + (cPath == null ? "/" : cPath);
    document.cookie = cName + "=" + value + ";expires=" + expire.toGMTString() + domain + path;
}
$(document).ready(function() {

    /*
     * Setup
     */
    var multipleQuestions = false;
    var questions = [];
    var indexQuestions = 0;
    var nbGoodQuestions = 0;
    var nbQuestion = 0;
    var currentMoveElement;
    var x = 0;
    var y = 0;
    var viewPadding = 15;
    var exerciseQuestion  = $(".exercise-question");
    var checkButton = $(".exercise-check");
    var nextQuestion = $(".exercise-next-question");
    var pageType = $(".quizview").length === 0 ? "exercise" : "quiz";
    var category = $(".breadcrumb li:last-child").attr("id");

    // URLs implementation
    if($("button.exercise-next-exercise").length !== 0) {
        $("button#back").attr("data-back", backPage);
        $("button.exercise-next-exercise").attr("data-next", nextPage);
    }
    /*
     * Setup Layout (container & view)
     */
    if($(".exercise-groups-bag").length !== 0 && !window.mobilecheck()){
        $(".exercise-groups-bag").each(function(){
            $(this).css('min-height', $(this).height());
            $(this).height("auto");
        })
    }

    if($(".exercise-gapfill-drag-bag").length !== 0 && !window.mobilecheck()){
        $(".exercise-gapfill-drag-bag").each(function(){
            $(this).height($(this).height());
        })
    }

    var widthExercise = $(".exercise-exercise").width();
    if (exerciseQuestion.length > 1){
        exerciseQuestion.width(widthExercise - 15);
        exerciseQuestion.hide();
        exerciseQuestion.first().show();
        $(".view").width(widthExercise);
        multipleQuestions = true;
    }

    // PLLDOCE-489 - Vérification/Check' button should be greyed out before any answers have been added
    if(!multipleQuestions && ($(".exercise-gap").length == 0 && $("select").length == 0)) 
        checkButton.hide();

    exerciseQuestion.each(function() {
        questions.push($(this));
    });

    var selectors = [ ".exercise-mc-dropdown select", ".exercise-gapfill .exercise-gap",
            ".exercise-groups-bag .exercise-item", ".exercise-gap-drag.undropped" ];
    $(selectors.join(",")).each(function() {
        nbQuestion++;
        if (this.tagName == "SELECT")
            this.selectedIndex = -1;
    });

    // display score max
    if(!multipleQuestions){
        $(".progression, #noDisplay, .exercise-next-question").hide();
        $(".nbAnswers").text("1");
        $(".outOf").text(($(".exercise-mc").length == 0) ? nbQuestion : "1");
        if($(".exercise-question").length !== 0){
            $(".exercise-question").css("display", "block");
        }
    } else {
        document.addEventListener("keyup", function(event){
            if(event.keyCode  ===  13 ){
                checkButton.show(); 
                checkButton.click();
            }
        });
        if ($(".exercise-question").find(".exercise-gapfill").length > 0)
            $(".outOf").text($(".exercise-question .exercise-gap-input").length);
        else
            $(".outOf").text(exerciseQuestion.length);
    }

    // sort randomly multiple choices
    if ($(".exercise-mc").length !== 0) {
        if (questions[indexQuestions].find(".exercise-mc").length !== 0
                && questions[indexQuestions].find(".exercise-choices").attr("data-randomize") !== "false") {
            shuffle(".exercise-choices");
            checkButton.hide();
        }
    }

    if ($(".exercise-mc-dropdown").length !== 0 && multipleQuestions) {
        if ($(questions[indexQuestions]).find(".exercise-mc-dropdown").length !== 0) {
            checkButton.hide();
        }
    }

    checkButton.prop('disabled', true);

    function mobileMove(e, _this) {
        e.preventDefault();
        var _self = $(_this);
        if (_self.prop("draggable") == false)
            return;

        var w = _self.width();
        var h = _self.height();

        x = e.originalEvent.changedTouches[0].clientX;
        y = e.originalEvent.changedTouches[0].clientY;

        $(_this).parent().height($(_this).parent().height()); // fix the height of the exercise-group-bag

        _self.addClass("currentMove");
        currentMoveElement = _self;
        _self.css({
            top : y - (h),
            left : x - (w / 2)
        });
        $(currentMoveElement).parent().removeClass("dropped").addClass("undropped");
    }

    function mobileClick(e, _this) {
        if (!currentMoveElement)
            return;

        if ((e.target.className === "exercise-gap-item" || e.target.className === "exercise-item")
                && $(e.target).parent().hasClass("exercise-gap-drag")
                && currentMoveElement.parent().hasClass("exercise-gap-drag")) {
            currentMoveElement.parent().removeClass("dropped").addClass("undropped");
            $(e.target).parent().removeClass("dropped").addClass("undropped");

            var fluentBox = e.target.parentNode;
            currentMoveElement[0].parentNode.appendChild(e.target);
            fluentBox.appendChild(currentMoveElement[0]);

            currentMoveElement.parent().removeClass("undropped").addClass("dropped");
            $(e.target).parent().removeClass("undropped").addClass("dropped");
        } else
            $(_this).append(currentMoveElement);
        currentMoveElement = null;
    }

    function mobileEnd(e, _this){
        x = e.originalEvent.changedTouches[0].clientX;
        y = e.originalEvent.changedTouches[0].clientY;

        $(".currentMove").removeClass("currentMove");
        var  h = $(_this).parents(".exercise-groups").height();
        $(_this).parents(".exercise-groups").find(document.elementFromPoint(x, y)).height($(document.elementFromPoint(x, y)).height());
        $(document.elementFromPoint(x, y)).click();
        $(_this).parents(".exercise-groups").find(".exercise-group").height("");

        var h2 = $(_this).parents(".exercise-groups").height();
        if(h != h2)
            $(".view").height($(".view").height() + h2 - h);

        // release the height of the exercise-group-bag
        $(".exercise-groups-bag,.sequence-bag-drag ").height("");

        // enable the check button
        var gapfillDrags = $(_this).closest(".exercise-question").find(".sequence-bag-drag");
        if (gapfillDrags.length == 1) {
            if(checkIfFilled($(gapfillDrags))) {
                checkButton.prop('disabled', false);
                nextQuestion.prop('disabled', true);
            } else {
                checkButton.prop('disabled', true);
            }
        }
    }

    $(".exercise-item").on(
        {"click": function(e){ 
            if($(this).parent(".exercise-sequence-drag").length !== 0)
                $(this).parent(".exercise-sequence-drag").click();
            if($(this).parent(".sequence-bag-drag").length !== 0)
                $(this).parent(".sequence-bag-drag").click();
        }
    });

    $(".exercise-gapfill").on('input', function(){
        updateCheckButton($(this).find("input"));
    });

    $(".exercise-group, .exercise-groups-bag, .exercise-sequence-drag ,.exercise-gapfill-drag-bag ,.exercise-gap-drag").on(
        {"click" : function(e) {
            if ($(this).hasClass("exercise-gap-drag") && $(this).children().length === 1 && currentMoveElement) {
                var offsetTop = $(this).offset().top;
                currentMoveElement.parent().removeClass("dropped").addClass("undropped");
                $(this).append(currentMoveElement);
                $(this).removeClass("undropped").addClass("dropped");
                currentMoveElement = null;
                var newoffsetTop = $(this).offset().top;
                if (offsetTop != newoffsetTop)
                    $(".view").height($(".view").height() + newoffsetTop - offsetTop)
            } else {
                mobileClick(e, this);
            }
        }
    });

    $(".exercise-groups .exercise-item, .sequence-bag-drag .exercise-item, .exercise-gapfill-drag-bag .exercise-gap-item").on(
        {"touchmove": function(e) { mobileMove(e, this) }
    });

    $(".exercise-groups .exercise-item, .exercise-groups .exercise-item, .sequence-bag-drag .exercise-item, .exercise-gapfill-drag-bag .exercise-gap-item").on(
        {"touchend": function(e) {
            if ($(this).prop("draggable") == false) {
                e.preventDefault();
                return;
            }

            if($(this).parent(".exercise-gapfill-drag-bag").length !== 0){
                x = e.originalEvent.changedTouches[0].clientX;
                y = e.originalEvent.changedTouches[0].clientY;
                $(".currentMove").removeClass("currentMove");

                if($(document.elementFromPoint(x, y)).hasClass("exercise-gap-drag") 
                        || $(document.elementFromPoint(x, y)).hasClass("exercise-gapfill-drag-bag") )
                            document.elementFromPoint(x, y).click();
                else if($(document.elementFromPoint(x, y)).hasClass("exercise-gap-item") 
                        && $(document.elementFromPoint(x, y)).parent().hasClass("exercise-gapfill-drag-bag"))
                            $(document.elementFromPoint(x, y)).parent().click();
                else if($(document.elementFromPoint(x, y)).hasClass("exercise-gap-item"))
                    $(currentMoveElement).parent().removeClass("undropped").addClass("dropped");
                else if($(document.elementFromPoint(x, y)).hasClass("exercise-item")
                        && $(document.elementFromPoint(x, y)).parent().hasClass("exercise-groups-bag"))
                    $(currentMoveElement).parent().removeClass("undropped").addClass("dropped");
                else
                    currentMoveElement = null;

                // remove mobile drag element container
                $(".exercise-gapfill-drag-bag").height("");
                // enable check button
                var gapfillDrags = $(this).closest(".exercise-question").find(".exercise-gapfill-drag-bag");
                if (gapfillDrags.length == 1 && checkIfFilled($(gapfillDrags))) {
                    checkButton.prop('disabled', false);
                    nextQuestion.prop('disabled', true);
                } else {
                    checkButton.prop('disabled', true);
                }
            } else
                mobileEnd(e, this);

            if($(this).parent(".exercise-group").length !== 0 && $(this).parent().siblings(".exercise-groups-bag").get(0).childElementCount === 0)
                checkButton.prop('disabled', false);
        }
    });

    shuffle(".sequence-bag-drag");
    shuffle(".exercise-groups-bag");
    shuffle(".exercise-gapfill-drag-bag"); 

    if($(".exercise-context").attr("data-static") !== 0) {
        $(".exercise-context").addClass("static");
    }

    /*
     * Handle the input text focus loose (or blur)
     */
    $(".exercise-gap-input").focusout(function() {
        if (multipleQuestions)
            return;

        var isFilled = true;
        $(".exercise-gapfill").find("input").each(function() {
        //$(this).parent().find(".exercise-gapfill input").each(function() {
            isFilled = isFilled && checkIfFilled($(this));
        });
        if (isFilled)
            checkButton.show();
    });

    /*
     * Handle the mc-dropdown event with or without multiple questions
     */
    $(".select-question").change(
        function(value, text) {
            if (multipleQuestions) {
                nextQuestion.prop("disabled", true);
                updateAnswers(exerciseCheck(questions[$(this).attr("question")]));
                changeQuestion();
                if (Math.floor($(".nbAnswers").text()) - 1 === questions.length) {
                    $(".progression").hide();
                    $(".exercise-replay").show();
                    $(".result").text(Math.floor($(".goodAnswers").text()) + " / " + Math.floor($(".nbAnswers").text()) - 1);
                }
            } else
                updateCheckButton($(this).siblings( "select"));
        });

    /*
     * Handle the mc click event
     */
    $(".exercise-choice-radio").click(function() {
        var idxQuestion = $(this).attr("question");
        if(questions[idxQuestion].resolve === true)
            return false;

        questions[idxQuestion].resolve = true;

        updateAnswers(exerciseCheck(questions[idxQuestion]));
        if(multipleQuestions) {
            nextQuestion.prop("disabled", true);
            changeQuestion($(this));
            indexQuestions++;
        } else {
            setTimeout(function(){
                exerciseQuestion.hide();
                $(".progression, .exercise-replay").show();
            }, 1700);
        }
    });

    /*
     * Handle the check my answer button
    */
    checkButton.click(function() {
        var isFilled = true;
        var selectors=[".exercise-gapfill input",".exercise-groups-bag",".sequence-bag-drag", ".exercise-gapfill-drag-bag"];
        questions[indexQuestions].find($(selectors.join(","))).each(
            function(){
                isFilled = isFilled && checkIfFilled($(this));
            }
        );

        if (! isFilled) 
            return;

        updateAnswers(exerciseCheck(questions[indexQuestions]));
        checkButton.hide();

        if(!multipleQuestions) {
            $(".exercise-replay, .progression").show();
            if($(".exercise-gapfill").length !== 0)
                $(".exercise-show").addClass("inline");
        } else if(questions[indexQuestions].find(".error").length != 0 && questions[indexQuestions].find(".exercise-gapfill").length == 1 ){
            $(".exercise-show").addClass("inline");
        } else {
            nextQuestion.prop("disabled", true);
            indexQuestions++;
            changeQuestion($(this));
        }

        sendGaEvent(pageType, 'check', category);
    });

    nextQuestion.click(function() {
    	var questionNb = $(".exercise-question:visible").length;
        nextQuestion.prop('disabled', true);
        $("#container").css({"pointer-events":"none"});
        updateAnswers(0);
        if (window.mobilecheck() && $(".exercise-gapfill-drag").length !== 0)
        	questions[questionNb - 1].find(".exercise-gapfill-drag-bag").hide();
        if (window.mobilecheck() && $(".exercise-groups").length !== 0)
            $($(".exercise-groups-bag")[indexQuestions]).hide();
        indexQuestions ++;
        changeQuestion($(this));
        $(".exercise-show").removeClass("inline");

        sendGaEvent(pageType, 'next', category);
    });

    /*
     * Handle the replay button
     */
    $(".exercise-replay").click(function() {
        $(".exercise-answer").hide();
        $(".exercise-check").prop('disabled', true);
        if(!multipleQuestions){
            reset();
            $(".progression").hide();
        } else
            location.reload();
        if($(".exercise-gap").length > 0 || $("select").length != 0)
            checkButton.show();
        $(".exercise-show").removeClass("inline");

        sendGaEvent(pageType, 'retry', category);
    });

    $(".exercise-show").click(function(){
        $(".exercise-gap-input").removeClass("check");
        displayAnswers(($(".exercise-gapfill").length !== 0 ? true: false));
    });

    if(window.hash !== ""){
        var id = window.location.hash.substring(1,window.location.hash.length);
        if(id != "")
        	$($("#"+id)[0]).addClass("current").parents(".items_content").show();
    }

    function reset(){
        var hasError = false;
        var selectors=[".exercise-mc-dropdown select",".exercise-gapfill .exercise-gap-input",".exercise-gap-drag .exercise-gap-item"];
        $(selectors.join(",")).each(
            function() {
                if ($(this).hasClass("error")) {
                    hasError = true;

                    $(this).prop("disabled", false);
                    $(this).prop("draggable", true);
                    $(this).removeClass("error");

                    if (this.tagName === "INPUT") {
                        this.value = "";
                    } else if (this.tagName === "DIV") {
                        $(this).parents(".exercise-gap-drag").removeClass("dropped").addClass("undropped");
                        $(this).parents(".exercise-question").children(".exercise-gapfill-drag-bag").append($(this));
                        $(this).find("i").remove();
                    } else if (this.tagName === "SELECT") {
                        $(this).prop('selectedIndex', -1);
                    }
                }
            });

        if(!hasError){
            location.reload();
        } else {
            $(".exercise-replay").hide();
            $(".goodAnswers").text("0");
        }
    }

    /*
     * Handle pretty much everything except mc-dropdown with multiple questions
     */
    function exerciseCheck(question) {
        var goodAnswer = 0;
        var onlyOneGoodAnswer = true;

        /*
         * Mc-dropdown
         */
        question.find(".exercise-mc-dropdown select").each(function() {
            $(this).addClass(this.value === $(this).find("option[data-correct]").val() ? "correct" : "error").prop("disabled", "disabled");

            onlyOneGoodAnswer = false;
            if ($(this).hasClass("correct") && !multipleQuestions)
                goodAnswer++;
        });

        /*
         * Gapfill
         */
        question.find(".exercise-gapfill .exercise-gap input").each(function() {
            $(this).prop("disabled", true);
            var c = $(this).parents().attr("data-correct");
            var formatedText = $(this).val().toLowerCase().replace(/^\s\s*/, "").replace(/\s\s*$/, "");
            $(this).parent().attr("data-input", formatedText);
            if (c.split("|") != c) {
                $(this).addClass(c.split("|").includes(formatedText) ? "correct" : "error check");
            } else
                $(this).addClass((c.toLowerCase() === formatedText) ? "correct" : "error check");

            onlyOneGoodAnswer = false;
            if ($(this).hasClass("correct"))
                goodAnswer++;
        });

        /*
         * Mc
         */
        question.find(".exercise-choice .exercise-choice-radio:checked").each(function() {
            $(this).addClass($(this).is("[data-correct]") ? "correct" : "error");
            putConfirmationSign($(this), "label");

            onlyOneGoodAnswer = false;
            if (!($(this).hasClass("error"))) 
                goodAnswer++;
        });

        /*
         * Sequence
         */
        question.find(".exercise-sequence-drag").children().each(function (index , item) {
            $(item).addClass(Math.floor($(item).attr("data")) === index +1 ? "correct" : "error");
            putConfirmationSign($(this), null);

            onlyOneGoodAnswer = onlyOneGoodAnswer && !($(item).hasClass("error"));
        });

        /*
         * Grouping
         */
        question.find(".exercise-groups > .exercise-group .exercise-item").each(function() {
            var parent_height = $(this).parent().height();
            $(this).prop("draggable", false);
            $(this).addClass($(this).attr("data-group") === $(this).parents(".exercise-group.canDrop").attr("id") ? "correct" : "error");
            putConfirmationSign($(this), null);
            var current_parent_height = $(this).parent().height();
            if(current_parent_height != parent_height) {
                $(".view").height($(".view").height() + current_parent_height - parent_height);
            }

            if (multipleQuestions) {
                onlyOneGoodAnswer = onlyOneGoodAnswer && $(this).hasClass("correct");
            } else {
                onlyOneGoodAnswer = false;
                if($(this).hasClass("correct"))
                    goodAnswer ++;
            }
        });

        /*
         * Gapfill drag - to test multiple
         */
        question.find(".exercise-gap > .exercise-gap-drag .exercise-gap-item").each(function() {
            $(this).prop("draggable", false);
            $(this).addClass($(this).text() === $(this).parents(".exercise-gap-drag").attr("data") ? "correct" : "error");
            putConfirmationSign($(this), null);

            if (multipleQuestions) {
                onlyOneGoodAnswer = onlyOneGoodAnswer && $(this).hasClass("correct");
            } else {
                onlyOneGoodAnswer = false;
                if($(this).hasClass("correct"))
                    goodAnswer ++;
                else
                    $(this).parent().siblings(".exercise-answer").show();
            }
        });

        if (onlyOneGoodAnswer)
            goodAnswer++;

        return goodAnswer;
    }

    $(".items_title").click(function(){
        var show = $(this).parent().next("div");

        if(show[0].localName != "br" )
            (show.is(":visible") ? (show.hide() && $($(this)[0].previousElementSibling).removeClass("fa-caret-down") && $($(this)[0].previousElementSibling).addClass("fa-caret-right"))
                                                         : (show.show() && $($(this)[0].previousElementSibling).removeClass("fa-caret-right") && $($(this)[0].previousElementSibling).addClass("fa-caret-down")));

        if(this.id === "collocations" || this.id === "synonyms"){
            if(!$(show).find("div:first").is(":visible"))
                $(show).find("a:first").click();
        }
    });

    $(".tocContent .dropdown-icon").click(function(){
        $($(this)[0].nextElementSibling).click();
    });

    $(".exercise-next-exercise").click(function(){
        location.href= $(this).attr("data-next");
    });

    $("#back").click(function(){
        location.href=$(this).attr("data-back");
    });
    
    window.addEventListener("DOMContentLoaded", function() {
    	if (location.hash != "") {
	    	var array = Array.from($("div.items_content"));
	    	array = array.filter(function(e) {if (e.style.display == "block") return e});
	    	array.map(function(e) {$(e.previousElementSibling.children["dot"]).removeClass("fa-caret-right").addClass("fa-caret-down")});
    	}
    });

    function changeQuestion(o) {
        setTimeout(function(myO) {
            var qestionNb = $(".exercise-question:visible").length;
            var container = $("#container");
            container.css("pointer-events","auto");

            if($(myO).hasClass("exercise-question")) {  // USED?
                var idxQuestion = $(myO).parents(".exercise-question").index(".exercise-question") + 1;
                if(exerciseQuestion.length > idxQuestion) {
                    container.css({"left":- $($(myO).parents(".exercise-question").find(".exercise-question")
                                                           .get(idxQuestion)).show().position().left });
                    $(".view").height($(exerciseQuestion[qestionNb]).height() + 46);
                } else {
                   container.css({"left":- container.children().last().show().position().left });
                }
            } else if(exerciseQuestion.length > indexQuestions) {
                if (questions[qestionNb].find(".exercise-mc").length !== 0 || questions[qestionNb].find(".exercise-mc-dropdown").length !== 0){
                    checkButton.hide();
                } else {
                    checkButton.prop('disabled', true);
                    checkButton.show();
                }
                container.css({"left":-$($(myO).parents(".exercise-exercise").find(".exercise-question").get(indexQuestions)).show().position().left});
                $(".view").height($(exerciseQuestion[qestionNb]).height() + 46);
            } else {
               container.css({"left":-container.children().last().show().position().left});
            }

            if(indexQuestions === exerciseQuestion.length){
                $(".exercise-rubric,.exercise-check,.progression,.exercise-next-question").hide();
                $(".finalResult").show();
                $(".exercise-replay").addClass("inline");
                $(".result").text(Math.floor($(".goodAnswers").text()) + " / " + Math.floor($(".outOf").text()));
                $(".view").height($(".finalResult").height() + 46);
                if(($(".exercise-mc-dropdown").length !== 0 || $(".exercise-mc").length !== 0) || $(".exercise-groups").length !== 0 || $(".exercise-sequence").length !== 0 && $(".exercise-question").length > 1) {
                    $(".exercise-correct").show();
                }
            } else {
                $(".exercise-next-question").show();
            }
            nextQuestion.prop('disabled', false);

            if ($(".exercise-gapfill").length !== 0){
                setTimeout(function(){
                    $($(".exercise-gapfill:visible").last()).find("input").focus();
                    nextQuestion.prop('disabled', false);
                }, 800);
            }
        }.bind(this,o), 1700);

        if ($(".exercise-mc").length !== 0){
            setTimeout(function(){
                $($(".exercise-audioAsset")[indexQuestions]).children().children().click();
            }, 2400);
        }
    }

    $(".exercise-correct").click(function(el){
        $(".exercise-choice-radio").prop('disabled', true);
        $(".exercise-question").css("display","block");
        $(".container").css("position", "static").css("overflow", "initial");
        $(".view").addClass("review").css("overflow", "initial").css("height", "");
        $(".exercise-question .exercise-mc, .exercise-question .exercise-mc-dropdown ").each(function(){
            var correct = $(this).find("[data-correct]");
            if(!correct.hasClass("correct"))
                correct.prop("checked", true).addClass("correct")
        });
        $(".exercise-question .exercise-sequence").each(function(){
            var showAnswer = false;
            $(this).find("[data]").each(
                function(){
                    if (!$(this).hasClass("correct")){
                        showAnswer = true;
                        return false;
                    }
                }
            );
          if(showAnswer)
              $(this).find('.exercise-answer').show();
        });

        $(".exercise-groups").each(function(){
            if($(this).find(".exercise-groups-bag").find(".exercise-item").length != 0)
                $(this).find(".exercise-groups-bag").addClass("space");
            var showAnswer = false;
            $(this).find("[data-group]").each(
                function(){
                    if (!$(this).hasClass("correct")){
                        showAnswer = true;
                        return false;
                    }
                }
            );
          if(showAnswer)
              $(this).find('.exercise-answer').show();
        });

        $(".exercise-gap > .exercise-gap-drag").each(function(){
            if (!$(this).find(".exercise-gap-item").hasClass("correct")){
                $(this).siblings(".exercise-answer").show();
            }
        });

        // prevent drag/drop
        $(".canDrop").each(function() {
            $(this).removeClass("canDrop");
            $(this).removeAttr("ondrop");
        });
        $('[draggable]').each(function() {
            $(this).prop("draggable", false);
            $(this).removeAttr("ondragstart");
        });
        $('[ondragover]').each(function() {
            $(this).removeAttr("ondragover");
        });

        sendGaEvent(pageType, 'review', category);
        $(this).hide();
    });
    function updateAnswers(answer){
        var goodAnswer = Math.floor($(".goodAnswers").text());
        nbAnswer = indexQuestions + 1;
        if(answer !== 0){
            goodAnswer += answer;
            $(".goodAnswers").text(goodAnswer);
        }

        if(multipleQuestions && nbAnswer < questions.length) {
            setTimeout(function(nbAnswer) {
                $(".nbAnswers").text(nbAnswer);
            }, 1700, nbAnswer + 1);
        }
    }

    setTimeout(function(){
        $(".view").height($(exerciseQuestion[0]).height() + 46);
    }, 150);

    function displayAnswers(gapfill) {
        if (gapfill) {
            //  to display all gapfill question and answers
            if (multipleQuestions) {
                $(exerciseQuestion[indexQuestions]).find(".exercise-gap").each(function() {
                    displayAnswer($(this));
                });
            } else {
                $(".exercise-gap").each(function() {
                    displayAnswer($(this));
                });
            }

            $(".exercise-show").removeClass("inline");
        } else {
            var question = $($(".exercise-gapfill-drag:visible").last());
            question.find(".exercise-gap-drag").each(
                function(el) {
                    this.appendChild($("[data-bag='" + this.attributes.data.value.replace("'", "\\'") + "']")[0]);
                });
            question.find(".exercise-gap-item").removeClass("error");
        }
        $(".exercise-show").removeClass("inline");
    }

    function updateCheckButton(items) {
        var disableCheck = false;
        items.each(function(){
            if ($(this).val().trim() === ""){
                disableCheck = true;
                return false;
            }
        });
        checkButton.prop('disabled', disableCheck);
    }
});

function checkIfFilled(obj) {
    var isFilled;
    if (obj.get(0).tagName === "INPUT") {
        isFilled = (obj.val() !== "") ;
    } else if (obj.hasClass("exercise-gapfill-drag-bag") && obj.parent().find("[data-distractor]").length !== 0) {
        isFilled = (obj.parent().find(".exercise-gap-drag").children(".exercise-gap-item").length === obj.parent()
                .find(".exercise-gap-drag").length);
    } else if (obj.hasClass("exercise-gapfill-drag-bag") && obj.parent().find($('[id^="item-distract"]')).length !== 0) {
        isFilled = (obj.parent().find($('[id^="item-distract"]')).length === obj.children().length);
    } else {
        isFilled = (obj.children().length === 0);
    }
    return isFilled;
}

function displayAnswer(obj) {
    var answer = obj.attr("data-correct");
    if (answer.split("|") != answer)
        answer = answer.split("|")[0];
    obj.find("input").val(answer).prop("disabled", true);
}

/*
 * Handle the drag and drop on Desktop
 */
function allowDrop(ev) {
    ev.preventDefault();
}

function drag(ev) {
    if(ev.target.draggable)
        ev.dataTransfer.setData("dragId", ev.target.id);
    else
        ev.preventDefault();
}

function drop(ev) {
    ev.preventDefault();
    var dragId = ev.dataTransfer.getData("dragId");
    if (dragId) {
        var item = $("#" + ev.dataTransfer.getData("dragId"));

        /*
         * Handle the Grouping type
         */
        if (ev.target.className.includes("canDrop")) {
            $(item).parent().removeClass("dropped").addClass("undropped");
            ev.target.appendChild(document.getElementById(dragId));
        }

        /*
         * Handle the Gapfill-type
         */
        if (ev.target.className.includes("exercise-gap-drag")) {
            if (ev.target.childElementCount === 1 && item) {
                $(item).parent().hasClass("exercise-gapfill-drag-bag") ? null : $(item).parent().removeClass("dropped")
                        .addClass("undropped");
                ev.target.appendChild(document.getElementById(dragId));
                $(ev.target).removeClass("undropped").addClass("dropped");
                var gapfillDrags = ev.target.closest(".exercise-question").getElementsByClassName("exercise-gapfill-drag-bag");
                if (gapfillDrags.length == 1 && checkIfFilled($(gapfillDrags.item(0)))) {
                    $(".exercise-check").prop('disabled', false);
                    $(".exercise-next-question").prop('disabled', true);
                } else {
                    $(".exercise-check").prop('disabled', true);
                }
            }
        } else if (ev.target.className.includes("exercise-sequence-drag")) {
            var gapfillDrags = ev.target.closest(".exercise-question").getElementsByClassName("sequence-bag-drag");
            if (gapfillDrags.length == 1 && checkIfFilled($(gapfillDrags.item(0)))) {
                $(".exercise-check").prop('disabled', false);
                $(".exercise-next-question").prop('disabled', true);
            } else {
                $(".exercise-check").prop('disabled', true);
            }
        } else if (ev.target.className.includes("exercise-groups-bag")) {
            ev.target.appendChild(document.getElementById(dragId));
        } else if (ev.target.className.includes("exercise-group")
                && $(ev.target).siblings(".exercise-groups-bag").get(0).childElementCount === 0) {
            $(".exercise-check").prop('disabled', false);
            $(".exercise-next-question").prop('disabled', true);
        } else if (ev.target.className === "exercise-gap-item" || ev.target.className === "exercise-item") {
            if ($(ev.target).parent().hasClass("exercise-gapfill-drag-bag")
                    || $(ev.target).parent().hasClass("sequence-bag-drag")
                    || $(ev.target).parent().hasClass("exercise-groups-bag")) {
                $(item).parent().removeClass("dropped").addClass("undropped");
                $(ev.target).parent()[0].appendChild(document.getElementById(dragId));
            } else if ($(ev.target).parent().hasClass("exercise-gap-drag")
                    && $("#" + dragId).parent().hasClass("exercise-gap-drag")) {
                $(item).parent().removeClass("dropped").addClass("undropped");
                $(ev.target).parent().removeClass("dropped").addClass("undropped");

                var fluentBox = ev.target.parentNode;

                document.getElementById(dragId).parentNode.appendChild(ev.target);
                fluentBox.appendChild(document.getElementById(dragId));

                $(item).parent().removeClass("undropped").addClass("dropped");
                $(ev.target).parent().removeClass("undropped").addClass("dropped");
            }
        }
    }
}

function updateAnswers(answer) {
    var goodAnswer = Math.floor($(".goodAnswers").text());
    var nbAnswer = Math.floor($(".nbAnswers").text()) - 1;
    nbAnswer += $(".exercise-question:visible").length;
    if (answer !== 0) {
        goodAnswer += answer;
        $(".goodAnswers").text(goodAnswer);
    }
    setTimeout(function(nbAnswer) {
        $(".nbAnswers").text(nbAnswer + 1);

    }.bind(nbAnswer), 1700);
}

function shuffle(className) {
    var bags = $(className);
    bags.each(function(idx){
        var divs = $(bags[idx]).children();
        
        while (divs.length) {
            $(bags[idx]).append(divs.splice(Math.floor(Math.random() * divs.length), 1)[0]);
        }
    });
}

function putConfirmationSign(respNode, confNodeName){
    var confNode = (confNodeName == null? respNode :respNode.parent().find(confNodeName));
    if(confNode.find("i").length != 0)
        return;

    var node = document.createElement('i');
    if (respNode.hasClass("error")) {
        node.className = "fas fa-times-circle";
    } else {
        node.className = "fas fa-check";
    }
    confNode.append(node);
}

//function to detect if the user is a phone;
window.mobilecheck = function() {
    var check = false;
    (function(a) {
        if (/(android|bb\d+|meego).+mobile|avantgo|bada\/|blackberry|blazer|compal|elaine|fennec|hiptop|iemobile|ip(hone|od)|iris|kindle|lge |maemo|midp|mmp|mobile.+firefox|netfront|opera m(ob|in)i|palm( os)?|phone|p(ixi|re)\/|plucker|pocket|psp|series(4|6)0|symbian|treo|up\.(browser|link)|vodafone|wap|windows ce|xda|xiino/i.test(a)
            || /1207|6310|6590|3gso|4thp|50[1-6]i|770s|802s|a wa|abac|ac(er|oo|s\-)|ai(ko|rn)|al(av|ca|co)|amoi|an(ex|ny|yw)|aptu|ar(ch|go)|as(te|us)|attw|au(di|\-m|r |s )|avan|be(ck|ll|nq)|bi(lb|rd)|bl(ac|az)|br(e|v)w|bumb|bw\-(n|u)|c55\/|capi|ccwa|cdm\-|cell|chtm|cldc|cmd\-|co(mp|nd)|craw|da(it|ll|ng)|dbte|dc\-s|devi|dica|dmob|do(c|p)o|ds(12|\-d)|el(49|ai)|em(l2|ul)|er(ic|k0)|esl8|ez([4-7]0|os|wa|ze)|fetc|fly(\-|_)|g1 u|g560|gene|gf\-5|g\-mo|go(\.w|od)|gr(ad|un)|haie|hcit|hd\-(m|p|t)|hei\-|hi(pt|ta)|hp( i|ip)|hs\-c|ht(c(\-| |_|a|g|p|s|t)|tp)|hu(aw|tc)|i\-(20|go|ma)|i230|iac( |\-|\/)|ibro|idea|ig01|ikom|im1k|inno|ipaq|iris|ja(t|v)a|jbro|jemu|jigs|kddi|keji|kgt( |\/)|klon|kpt |kwc\-|kyo(c|k)|le(no|xi)|lg( g|\/(k|l|u)|50|54|\-[a-w])|libw|lynx|m1\-w|m3ga|m50\/|ma(te|ui|xo)|mc(01|21|ca)|m\-cr|me(rc|ri)|mi(o8|oa|ts)|mmef|mo(01|02|bi|de|do|t(\-| |o|v)|zz)|mt(50|p1|v )|mwbp|mywa|n10[0-2]|n20[2-3]|n30(0|2)|n50(0|2|5)|n7(0(0|1)|10)|ne((c|m)\-|on|tf|wf|wg|wt)|nok(6|i)|nzph|o2im|op(ti|wv)|oran|owg1|p800|pan(a|d|t)|pdxg|pg(13|\-([1-8]|c))|phil|pire|pl(ay|uc)|pn\-2|po(ck|rt|se)|prox|psio|pt\-g|qa\-a|qc(07|12|21|32|60|\-[2-7]|i\-)|qtek|r380|r600|raks|rim9|ro(ve|zo)|s55\/|sa(ge|ma|mm|ms|ny|va)|sc(01|h\-|oo|p\-)|sdk\/|se(c(\-|0|1)|47|mc|nd|ri)|sgh\-|shar|sie(\-|m)|sk\-0|sl(45|id)|sm(al|ar|b3|it|t5)|so(ft|ny)|sp(01|h\-|v\-|v )|sy(01|mb)|t2(18|50)|t6(00|10|18)|ta(gt|lk)|tcl\-|tdg\-|tel(i|m)|tim\-|t\-mo|to(pl|sh)|ts(70|m\-|m3|m5)|tx\-9|up(\.b|g1|si)|utst|v400|v750|veri|vi(rg|te)|vk(40|5[0-3]|\-v)|vm40|voda|vulc|vx(52|53|60|61|70|80|81|83|85|98)|w3c(\-| )|webc|whit|wi(g |nc|nw)|wmlb|wonu|x700|yas\-|your|zeto|zte\-/i.test(a.substr(0, 4)))
            check = true;
    })(navigator.userAgent || navigator.vendor || window.opera);
    return check;
};//translator.js
$(document).ready(function(){
    $(".translator-form textarea").on("input", function() {
        $(".translator-form .count").text($(this).val().length);
    });

    $(".translator-form *[name='from']").on("change", function() {
        controlSwitch();
    });

    $(".translator-form .from .dropdown-icon").on("click", function() {
        $(".from .se-text").trigger("click");
    });

    $(".translator-form .to .dropdown-icon").on("click", function() {
        $(".to .se-text").trigger("click");
    });

    function controlSwitch() {
        if ($(".translator-form *[name='from']").val() == "")
            $(".translator-form .switch").addClass('disabled');
        else
            $(".translator-form .switch").removeClass('disabled');
    }
    controlSwitch();

    var $from = $(".translator-form .from").data('Select');
    var $to = $(".translator-form .to").data('Select');
    var $charCount = $(".translator-form .count").text().trim();
    $(".translator-form .switch").on("click", function(e) {
        e.preventDefault();
        if (!$(this).hasClass('disabled')) {
            var from = $from.getValue();
            var to = $to.getValue();
            $from.setValue(to);
            $to.setValue(from);
        }
        sendGaEvent('Translator', 'button_swap_language', $charCount);
        return false;
    });
});

function sendGEvent() {
    var $optFrom = $(".translator-form .from .se-options").find(".se-selected");
    var $optTo = $(".translator-form .to .se-options").find(".se-selected");
    var langFrom = ($optFrom.attr("data-value") == "" ? "Auto" : $optFrom.attr("data-text-default").trim());
    var langTo = $optTo.attr("data-text-default").trim();
    var charCount = $('#translator-form .count').text().trim();

    sendGaEvent('Translator', langFrom + '_to_' + langTo, charCount);
    return true;
}

// Select
;(function($, window, document, undefined) {

	"use strict";

	// Create the defaults once
    var pluginName = "Select", defaults = {
        clsPrefixe : 'se-',
        name : null,
        options : [],
        value : null,
        defaultValue : null
		};

	// The actual plugin constructor
    function Select(element, options) {
        this.settings = $.extend({}, defaults, options);
		this._defaults = defaults;
		this._name = pluginName;

		var cls = [
			"select",
			"text",
			"value",
			"options",
			"option",

			"selected"
		];

		this.cls = {};
        for ( var i in cls)
			this.cls[cls[i]] = this.settings.clsPrefixe + cls[i];

		this.sel = {};
        for ( var clazz in this.cls)
			this.sel[clazz] = '.' + this.cls[clazz];

		this.map();

		this.initialValue = null;
        if (this.index.hasOwnProperty(this.settings.value))
			this.initialValue = this.settings.value;
        else if (this.index.hasOwnProperty(this.settings.defaultValue))
			this.initialValue = this.settings.defaultValue;
		else
			this.initialValue = this.settings.options[0].code;

		this.$element = element.addClass(this.cls.select);
		this.$label = $('<div class="' + this.cls.text + '"></div>').appendTo(this.$element);
        $('<i class="fas fa-caret-down dropdown-icon" aria-hidden="true"></i>').appendTo(this.$element);
		this.$value = $('<input class="' + this.cls.value + '" name="' + this.settings.name + '" type="hidden" />').appendTo(this.$element);

		this.$options = $('<div class="' + this.cls.options + '"></div>').appendTo(this.$element);

		this.init();
	}

	// Avoid Plugin.prototype conflicts
    $.extend(Select.prototype, {

        init : function() {
			var me = this;

            $.each(this.settings.options, function(i, option) {
                var divAttr = 'class="' + me.cls.option + '" data-value="' + option.code + '" data-text-default="' + option.defName + '"';
                $('<div ' + divAttr + '>' + option.name + '</div>').appendTo(me.$options);
			});
			this.setValue(this.initialValue);

			me.$label.on('click', function(e) {
				me.$options.toggle();
			});

			me.$options.on('click', 'div', function(e) {
                me.setValue($(this).attr("data-value"));
			});
		},

        map : function() {
			var me = this;
			me.index = {};
			me.reverse = {};
            $.each(me.settings.options, function(i, option) {
				me.index[option.code] = option.name;
				me.reverse[option.name] = option.code;
			});
		},

        setValue : function(value) {
			this.$options.find(this.sel.selected).removeClass(this.cls.selected);
			this.$options.find("*[data-value='" + value + "']").addClass(this.cls.selected);
			this.$options.hide();
			this.$label.text(this.index[value]);
			this.$value.val(value);
			this.$value.trigger('change');
		},

        getValue : function(value) {
			return this.$value.val();
		}
	});

	// A really lightweight plugin wrapper around the constructor,
	// preventing against multiple instantiations
    $.fn[pluginName] = function(options) {
        return this.each(function() {
            if (!$.data(this, pluginName)) {
                $.data(this, pluginName, new Select($(this), options));
			}
        });
	};

})(jQuery, window, document);
`
